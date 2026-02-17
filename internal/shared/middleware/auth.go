package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AuthContextKey is the key used to store user info in context
const (
	AuthContextKey = "user_id"
	RoleContextKey = "user_roles"
)

// AuthMiddleware validates JWT tokens and extracts user information
// TODO: Implement actual JWT validation with a proper library (e.g., golang-jwt/jwt)
func AuthMiddleware() gin.HandlerFunc {
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

		// TODO: Validate JWT token and extract claims
		// For now, this is a placeholder implementation
		userID, err := validateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// Store user ID in context
		c.Set(AuthContextKey, userID)

		c.Next()
	}
}

// OptionalAuthMiddleware is similar to AuthMiddleware but doesn't abort if token is missing
func OptionalAuthMiddleware() gin.HandlerFunc {
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
		userID, err := validateToken(token)
		if err == nil {
			c.Set(AuthContextKey, userID)
		}

		c.Next()
	}
}

// validateToken validates the JWT token and returns the user ID
// TODO: Implement actual JWT validation logic
func validateToken(token string) (uuid.UUID, error) {
	// Placeholder implementation
	// In production, you would:
	// 1. Parse the JWT token
	// 2. Verify the signature
	// 3. Check expiration
	// 4. Extract claims (user_id, roles, etc.)

	// For now, return a dummy UUID
	return uuid.Nil, nil
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