package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Context keys for storing auth info
const (
	AuthContextKey   = "user_id"
	DeviceContextKey = "device_id"
	RoleContextKey   = "user_roles"
)

// TokenValidator is an interface for validating JWT access tokens
type TokenValidator interface {
	ValidateAccessToken(tokenString string) (userID uuid.UUID, deviceID string, roles []string, err error)
}

// AuthMiddleware validates JWT tokens and extracts user information
func AuthMiddleware(validator TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]

		userID, deviceID, roles, err := validator.ValidateAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Store claims in context
		c.Set(AuthContextKey, userID)
		c.Set(DeviceContextKey, deviceID)
		c.Set(RoleContextKey, roles)

		c.Next()
	}
}

// OptionalAuthMiddleware is similar to AuthMiddleware but doesn't abort if token is missing
func OptionalAuthMiddleware(validator TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		token := parts[1]
		userID, deviceID, roles, err := validator.ValidateAccessToken(token)
		if err == nil {
			c.Set(AuthContextKey, userID)
			c.Set(DeviceContextKey, deviceID)
			c.Set(RoleContextKey, roles)
		}

		c.Next()
	}
}

// GetUserID retrieves the user ID from the Gin context
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get(AuthContextKey)
	if !exists {
		return uuid.Nil, false
	}

	id, ok := userID.(uuid.UUID)
	return id, ok
}

// GetDeviceID retrieves the device ID from the Gin context
func GetDeviceID(c *gin.Context) (string, bool) {
	deviceID, exists := c.Get(DeviceContextKey)
	if !exists {
		return "", false
	}

	id, ok := deviceID.(string)
	return id, ok
}
