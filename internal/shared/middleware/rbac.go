package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PermissionChecker is an interface for checking user permissions
type PermissionChecker interface {
	HasPermission(ctx context.Context, userID uuid.UUID, permissionSlug string) (bool, error)
}

// RBACMiddleware creates middleware that checks if the user has required permissions
func RBACMiddleware(checker PermissionChecker, requiredPermissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract user ID from context (should be set by AuthMiddleware)
		userID, exists := GetUserID(c)
		if !exists || userID == uuid.Nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}

		// Check each required permission
		for _, permission := range requiredPermissions {
			hasPermission, err := checker.HasPermission(c.Request.Context(), userID, permission)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check permissions"})
				c.Abort()
				return
			}

			if !hasPermission {
				c.JSON(http.StatusForbidden, gin.H{
					"error":      "insufficient permissions",
					"permission": permission,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// RequireAnyPermission checks if the user has at least one of the specified permissions
func RequireAnyPermission(checker PermissionChecker, permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := GetUserID(c)
		if !exists || userID == uuid.Nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			c.Abort()
			return
		}

		// Check if user has any of the permissions
		for _, permission := range permissions {
			hasPermission, err := checker.HasPermission(c.Request.Context(), userID, permission)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check permissions"})
				c.Abort()
				return
			}

			if hasPermission {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":       "insufficient permissions",
			"permissions": permissions,
		})
		c.Abort()
	}
}