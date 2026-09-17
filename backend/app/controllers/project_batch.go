package controllers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/db/d1http"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services/contentflag"
)

const maxBatchProjects = 20

type BatchProjectInput struct {
	Name      string `json:"name"`
	Framework string `json:"framework"`
}

type BatchProjectResult struct {
	Project *models.Project
	Status  string
}

type batchValidationError struct{ Message string }

func (e *batchValidationError) Error() string { return e.Message }

func validateProjectName(name string) string {
	nameLen := utf8.RuneCountInString(name)
	if nameLen < 1 || nameLen > 100 {
		return "Project name must be between 1 and 100 characters"
	}
	if !projectNameRegex.MatchString(name) {
		return "Project name can only contain letters, numbers, spaces, hyphens, and underscores"
	}
	return ""
}

const invalidFrameworkMessage = "Framework must be one of: gin, fiber, chi, fasthttp, stdlib, custom, react, svelte, vuejs, nextjs, nestjs, express, remix, jquery, react-native, hono, cloudflare, opentelemetry, symfony, laravel, django, flutter, android, ios"

func batchCreateProjects(tx *sql.Tx, orgId int, createdBy int, inputs []BatchProjectInput) ([]BatchProjectResult, error) {
	if len(inputs) == 0 {
		return nil, &batchValidationError{"At least one project is required"}
	}
	if len(inputs) > maxBatchProjects {
		return nil, &batchValidationError{"At most 20 projects per request"}
	}

	for i := range inputs {
		inputs[i].Name = strings.TrimSpace(inputs[i].Name)
		if msg := validateProjectName(inputs[i].Name); msg != "" {
			return nil, &batchValidationError{msg}
		}
		if !validFrameworks[inputs[i].Framework] {
			traceway.CaptureMessage("Invalid framework received in batch create: " + inputs[i].Framework)
			return nil, &batchValidationError{invalidFrameworkMessage}
		}
	}

	existing, err := transactional.ProjectRepository.FindByOrganizationId(tx, orgId)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*models.Project, len(existing))
	for _, p := range existing {
		byName[p.Name] = p
	}

	results := make([]BatchProjectResult, 0, len(inputs))
	for _, input := range inputs {
		if project, ok := byName[input.Name]; ok {
			if project.Framework != input.Framework {
				return nil, &batchValidationError{Message: "Project " + input.Name + " already exists with framework " + project.Framework + "; requested " + input.Framework}
			}
			results = append(results, BatchProjectResult{Project: project, Status: "existing"})
			continue
		}

		if ProjectLimitHook != nil {
			if err := ProjectLimitHook(tx, orgId); err != nil {
				return nil, err
			}
		}

		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, input.Name, input.Framework, orgId)
		if err != nil {
			return nil, err
		}
		if err := populateDefaultDashboards(tx, project, &createdBy); err != nil {
			return nil, err
		}
		byName[project.Name] = project
		results = append(results, BatchProjectResult{Project: project, Status: "created"})
	}
	return results, nil
}

type BatchProjectResponseItem struct {
	Id             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Framework      string    `json:"framework"`
	Token          string    `json:"token"`
	SourceMapToken *string   `json:"sourceMapToken,omitempty"`
	BackendUrl     string    `json:"backendUrl"`
	Status         string    `json:"status"`
}

func toBatchProjectResponse(results []BatchProjectResult) []BatchProjectResponseItem {
	items := make([]BatchProjectResponseItem, 0, len(results))
	for _, r := range results {
		items = append(items, BatchProjectResponseItem{
			Id:             r.Project.Id,
			Name:           r.Project.Name,
			Framework:      r.Project.Framework,
			Token:          r.Project.Token,
			SourceMapToken: r.Project.SourceMapToken,
			BackendUrl:     models.GetBackendUrl(),
			Status:         r.Status,
		})
	}
	return items
}

func cacheCreatedProjectsOnCommit(ctx *gin.Context, results []BatchProjectResult) {
	created := []*models.Project{}
	for _, r := range results {
		if r.Status == "created" {
			created = append(created, r.Project)
		}
	}
	if len(created) == 0 {
		return
	}
	middleware.OnCommit(ctx, func() {
		for _, p := range created {
			cache.ProjectCache.AddProject(p)
		}
	})
}

func respondBatchCreateError(ctx *gin.Context, err error) {
	var validationErr *batchValidationError
	if errors.As(err, &validationErr) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": validationErr.Message})
		return
	}
	var limitErr *LimitExceededError
	if errors.As(err, &limitErr) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": limitErr.Message})
		return
	}
	ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("batch create projects: %w", err))
}

type BatchCreateProjectsRequest struct {
	OrganizationId int                 `json:"organizationId" binding:"required"`
	Projects       []BatchProjectInput `json:"projects" binding:"required"`
}

func (p projectController) BatchCreateProjects(ctx *gin.Context) {
	var request BatchCreateProjectsRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}

	if db.IsCloudflare() {
		p.batchCreateProjectsCloudflare(ctx, request)
		return
	}

	tx := db.GetTx(ctx)
	if !requireOrgWrite(ctx, tx, request.OrganizationId) {
		return
	}

	results, err := batchCreateProjects(tx, request.OrganizationId, middleware.GetUserId(ctx), request.Projects)
	if err != nil {
		respondBatchCreateError(ctx, err)
		return
	}

	cacheCreatedProjectsOnCommit(ctx, results)
	ctx.JSON(http.StatusCreated, gin.H{"projects": toBatchProjectResponse(results)})
}

func (p projectController) batchCreateProjectsCloudflare(ctx *gin.Context, request BatchCreateProjectsRequest) {
	if len(request.Projects) == 0 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "At least one project is required"})
		return
	}
	if len(request.Projects) > maxBatchProjects {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "At most 20 projects per request"})
		return
	}

	userID := middleware.GetUserId(ctx)
	role, err := transactional.OrganizationRepository.GetUserRole(db.DB, request.OrganizationId, userID)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("batch create projects: load organization role: %w", err))
		return
	}
	if role == "" {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}
	if role == "readonly" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "You have read-only access to this organization"})
		return
	}
	if ProjectLimitHook != nil {
		ctx.AbortWithError(http.StatusInternalServerError, errors.New("Cloudflare project limits require a D1 implementation"))
		return
	}

	existing, err := transactional.ProjectRepository.FindByOrganizationId(db.DB, request.OrganizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("batch create projects: load existing projects: %w", err))
		return
	}
	byName := make(map[string]*models.Project, len(existing))
	for _, project := range existing {
		byName[project.Name] = project
	}

	now := time.Now().UTC()
	statements := make([]d1http.Statement, 0, len(request.Projects))
	results := make([]BatchProjectResult, 0, len(request.Projects))
	for i := range request.Projects {
		input := &request.Projects[i]
		input.Name = strings.TrimSpace(input.Name)
		if msg := validateProjectName(input.Name); msg != "" {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
			return
		}
		if !validFrameworks[input.Framework] {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": invalidFrameworkMessage})
			return
		}
		if existingProject, ok := byName[input.Name]; ok {
			if existingProject.Framework != input.Framework {
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Project " + input.Name + " already exists with framework " + existingProject.Framework + "; requested " + input.Framework})
				return
			}
			results = append(results, BatchProjectResult{Project: existingProject, Status: "existing"})
			continue
		}

		project := &models.Project{
			Id:                      uuid.New(),
			Name:                    input.Name,
			Token:                   strings.ReplaceAll(uuid.NewString(), "-", ""),
			Framework:               input.Framework,
			OrganizationId:          &request.OrganizationId,
			CreatedAt:               now,
			DropHealthyHealthchecks: true,
			ProfileLabelAllowlist:   models.StringSlice{},
			AiFlaggedTerms:          models.StringSlice{},
			AiFlaggedLanguages:      models.StringSlice(contentflag.DefaultLanguages),
		}
		if input.Framework == "ios" {
			token := strings.ReplaceAll(uuid.NewString(), "-", "")
			project.SourceMapToken = &token
		}
		statements = append(statements, d1http.Statement{
			SQL: `INSERT INTO projects (id, name, token, framework, organization_id, source_map_token, created_at, drop_healthy_healthchecks, profile_label_allowlist, ai_flagged_terms, ai_flagged_languages)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			Params: []any{project.Id, project.Name, project.Token, project.Framework, request.OrganizationId, project.SourceMapToken, project.CreatedAt, project.DropHealthyHealthchecks, project.ProfileLabelAllowlist, project.AiFlaggedTerms, project.AiFlaggedLanguages},
		})
		byName[project.Name] = project
		results = append(results, BatchProjectResult{Project: project, Status: "created"})
	}
	if _, err := db.BatchMain(ctx.Request.Context(), statements); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("batch create projects D1: %w", err))
		return
	}
	for _, result := range results {
		if result.Status == "created" {
			cache.ProjectCache.AddProject(result.Project)
		}
	}
	ctx.JSON(http.StatusCreated, gin.H{"projects": toBatchProjectResponse(results)})
}
