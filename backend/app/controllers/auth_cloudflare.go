package controllers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/db/d1http"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/services/contentflag"
)

func (a authController) loginCloudflare(c *gin.Context, request models.LoginRequest) {
	user, err := transactional.UserRepository.FindByEmail(db.DB, request.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if user == nil || !services.CheckPassword(request.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token, err := services.GenerateToken(user.Id, user.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	projects, err := transactional.ProjectRepository.FindAllWithBackendUrlByUserId(db.DB, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	organizations, err := transactional.OrganizationRepository.FindByUserIdWithRoles(db.DB, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, &models.LoginResponse{
		Token: token, User: user.ToResponse(), Projects: projects, Organizations: organizations,
	})
}

// registerCloudflare is the finite D1 equivalent of the native registration
// transaction. D1 batches execute the whole write set atomically, while reads
// after the batch use the normal database/sql connector.
func (a authController) registerCloudflare(c *gin.Context, request models.RegisterRequest) {
	wantsProject := request.ProjectName != "" || request.Framework != ""
	if wantsProject && (request.ProjectName == "" || request.Framework == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "projectName and framework must be provided together"})
		return
	}
	if wantsProject && !validFrameworks[request.Framework] {
		traceway.CaptureMessage("Invalid framework received during registration: " + request.Framework)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Framework must be one of: gin, fiber, chi, fasthttp, stdlib, custom, react, svelte, vuejs, jquery, react-native, hono, cloudflare, opentelemetry, symfony, flutter, android, ios"})
		return
	}

	exists, err := transactional.UserRepository.EmailExists(db.DB, request.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}

	hashedPassword, err := services.HashPassword(request.Password)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	now := time.Now().UTC()
	statements := []d1http.Statement{
		{SQL: "INSERT INTO users (email, name, password, created_at) VALUES (?, ?, ?, ?)", Params: []any{request.Email, request.Name, hashedPassword, now}},
		{SQL: "INSERT INTO organizations (name, timezone, created_at) VALUES (?, ?, ?)", Params: []any{request.OrganizationName, request.Timezone, now}},
		{SQL: "INSERT INTO organization_users (user_id, organization_id, role, created_at) VALUES ((SELECT id FROM users WHERE email = ?), last_insert_rowid(), 'owner', ?)", Params: []any{request.Email, now}},
	}

	var project *models.Project
	if wantsProject {
		project = &models.Project{
			Id:                      uuid.New(),
			Name:                    request.ProjectName,
			Token:                   uuid.NewString(),
			Framework:               request.Framework,
			CreatedAt:               now,
			DropHealthyHealthchecks: true,
			ProfileLabelAllowlist:   models.StringSlice{},
			AiFlaggedTerms:          models.StringSlice{},
			AiFlaggedLanguages:      models.StringSlice(contentflag.DefaultLanguages),
		}
		if request.Framework == "ios" {
			token := uuid.NewString()
			project.SourceMapToken = &token
		}
		statements = append(statements, d1http.Statement{
			SQL: `INSERT INTO projects (id, name, token, framework, organization_id, source_map_token, created_at, drop_healthy_healthchecks, profile_label_allowlist, ai_flagged_terms, ai_flagged_languages)
				VALUES (?, ?, ?, ?, (SELECT organization_id FROM organization_users WHERE user_id = (SELECT id FROM users WHERE email = ?)), ?, ?, ?, ?, ?, ?)`,
			Params: []any{project.Id, project.Name, project.Token, project.Framework, request.Email, project.SourceMapToken, project.CreatedAt, project.DropHealthyHealthchecks, project.ProfileLabelAllowlist, project.AiFlaggedTerms, project.AiFlaggedLanguages},
		})
	}

	results, err := db.BatchMain(c.Request.Context(), statements)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("register D1 batch: %w", err))
		return
	}
	if len(results) < 2 || results[0].LastInsertID == 0 || results[1].LastInsertID == 0 {
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("register D1 batch returned invalid insert ids"))
		return
	}

	user := &models.User{Id: int(results[0].LastInsertID), Email: request.Email, Name: request.Name, Password: hashedPassword, CreatedAt: now}
	organizationID := int(results[1].LastInsertID)
	if project != nil {
		project.OrganizationId = &organizationID
		cache.ProjectCache.AddProject(project)
	}

	token, err := services.GenerateToken(user.Id, user.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	projects, err := transactional.ProjectRepository.FindAllWithBackendUrlByUserId(db.DB, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	organizations, err := transactional.OrganizationRepository.FindByUserIdWithRoles(db.DB, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	var projectWithURL *models.ProjectWithBackendUrl
	if project != nil {
		projectWithURL = project.ToProjectWithBackendUrl()
		projectWithURL.Role = "owner"
	}
	c.JSON(http.StatusCreated, &models.RegisterResponse{
		Token: token, User: user.ToResponse(), Project: projectWithURL, Projects: projects, Organizations: organizations,
	})
}
