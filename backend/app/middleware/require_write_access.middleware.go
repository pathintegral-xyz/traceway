package middleware

import (
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

// RequireWriteAccess middleware checks if the user has write access to the project's organization.
// It blocks access for users with 'readonly' role.
// This middleware should be applied AFTER UseAppAuth.
var RequireWriteAccess gin.HandlerFunc

func InitRequireWriteAccess() {
	RequireWriteAccess = func(c *gin.Context) {
		userId := GetUserId(c)
		if userId == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		projectId := extractProjectId(c)
		if projectId == uuid.Nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		role, err := transactional.ProjectRepository.GetEffectiveRole(db.MainExecutor(c), projectId, userId)

		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to resolve effective role: %w", err))
			return
		}

		if role == "readonly" || role == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Read-only access. Write operations are not permitted."})
			return
		}

		c.Next()
	}
}
