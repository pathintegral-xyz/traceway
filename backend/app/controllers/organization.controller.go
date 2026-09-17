package controllers

import (
	"errors"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/oncall"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type organizationController struct{}

func (c *organizationController) GetSettings(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)
	userRole := middleware.GetUserOrgRole(ctx)

	type settingsData struct {
		Organization *models.Organization
		Members      []*models.OrganizationMember
		Invitations  []*models.InvitationWithInviter
	}

	org, err := transactional.OrganizationRepository.FindById(db.MainExecutor(ctx), organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load settings: %w", err))
		return
	}
	members, err := transactional.OrganizationRepository.GetMembersWithDetails(db.MainExecutor(ctx), organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load settings: %w", err))
		return
	}
	invitations, err := transactional.InvitationRepository.FindByOrganization(db.MainExecutor(ctx), organizationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load settings: %w", err))
		return
	}
	data := &settingsData{Organization: org, Members: members, Invitations: invitations}

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load settings: %w", err))
		return
	}

	if data.Organization == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Organization not found"})
		return
	}

	ctx.JSON(http.StatusOK, &models.OrganizationSettingsResponse{
		Organization: data.Organization,
		Members:      data.Members,
		Invitations:  data.Invitations,
		UserRole:     userRole,
	})
}

func (c *organizationController) GetMembers(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)

	members, err := transactional.OrganizationRepository.GetMembersWithDetails(db.MainExecutor(ctx), organizationId)

	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to load members: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, members)
}

type UpdateSettingsRequest struct {
	Timezone string `json:"timezone" binding:"required"`
}

func (c *organizationController) UpdateSettings(ctx *gin.Context) {
	organizationId := middleware.GetOrganizationId(ctx)
	tx := db.MainExecutor(ctx)

	var req UpdateSettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request")
		return
	}

	_, err := time.LoadLocation(req.Timezone)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid timezone"})
		return
	}

	err = transactional.OrganizationRepository.UpdateTimezone(tx, organizationId, req.Timezone)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("Failed to update settings: %w", err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"timezone": req.Timezone})
}

var OrganizationController = organizationController{}

type CreateOrganizationRequest struct {
	Name     string `json:"name" binding:"required"`
	Timezone string `json:"timezone"`
}

func (c *organizationController) Create(ctx *gin.Context) {
	var request CreateOrganizationRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(ctx, err, "Organization name is required")
		return
	}

	name := strings.TrimSpace(request.Name)
	if nameLen := utf8.RuneCountInString(name); nameLen < 1 || nameLen > 100 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Organization name must be between 1 and 100 characters"})
		return
	}

	timezone := strings.TrimSpace(request.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := oncall.LoadTimezone(timezone); err != nil {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	userId := middleware.GetUserId(ctx)
	tx := db.MainExecutor(ctx)

	// One organization per self-hosted instance; 422 rather than Register's 409 so the message reaches the recovery form.
	if config.Config.CloudMode != "true" {
		hasOrganizations, err := transactional.OrganizationRepository.HasOrganizations(tx)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check for existing organizations: %w", err))
			return
		}
		if hasOrganizations {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This instance already has an organization. Ask an administrator to invite you to it."})
			return
		}
	}

	if OrganizationLimitHook != nil {
		if err := OrganizationLimitHook(tx, userId); err != nil {
			var limitErr *LimitExceededError
			if errors.As(err, &limitErr) {
				ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": limitErr.Message})
				return
			}
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("organization limit hook failed: %w", err))
			return
		}
	}

	user, err := transactional.UserRepository.FindById(tx, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load creating user: %w", err))
		return
	}
	if user == nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("authenticated user %d not found", userId))
		return
	}

	org, err := transactional.OrganizationRepository.Create(tx, name, timezone)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create organization: %w", err))
		return
	}

	if _, err := transactional.OrganizationRepository.AddUser(tx, org.Id, user.Id, "owner"); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to add creator to organization: %w", err))
		return
	}

	for _, hook := range PostRegistrationHooks {
		if err := hook(tx, org, user); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("post-registration hook failed: %w", err))
			return
		}
	}

	ctx.JSON(http.StatusCreated, models.UserOrganizationResponse{
		Id:       org.Id,
		Name:     org.Name,
		Role:     "owner",
		Timezone: org.Timezone,
	})
}
