package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/voronka/backend/internal/auth"
	"github.com/voronka/backend/internal/shared/middleware"
	"go.uber.org/zap"
)

// HTTPHandler handles HTTP requests for auth operations
type HTTPHandler struct {
	usecase auth.Usecase
	logger  *zap.Logger
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(usecase auth.Usecase, logger *zap.Logger) *HTTPHandler {
	return &HTTPHandler{
		usecase: usecase,
		logger:  logger,
	}
}

// RegisterRoutes registers all auth routes
func (h *HTTPHandler) RegisterRoutes(router *gin.RouterGroup, authMw gin.HandlerFunc) {
	authGroup := router.Group("/auth")
	{
		// Public routes
		authGroup.POST("/register", h.Register)
		authGroup.POST("/login", h.Login)
		authGroup.POST("/refresh", h.Refresh)

		// Authenticated routes
		authGroup.POST("/logout", authMw, h.Logout)
		authGroup.POST("/logout-all", authMw, h.LogoutAll)
		authGroup.GET("/sessions", authMw, h.GetSessions)
		authGroup.DELETE("/sessions/:deviceId", authMw, h.RevokeSession)
	}
}

// Register handles POST /api/v1/auth/register
func (h *HTTPHandler) Register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenPair, err := h.usecase.Register(c.Request.Context(), &req, c.ClientIP())
	if err != nil {
		h.logger.Error("failed to register user", zap.Error(err))
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tokenPair)
}

// Login handles POST /api/v1/auth/login
func (h *HTTPHandler) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenPair, err := h.usecase.Login(c.Request.Context(), &req, c.ClientIP())
	if err != nil {
		h.logger.Error("failed to login", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tokenPair)
}

// Refresh handles POST /api/v1/auth/refresh
func (h *HTTPHandler) Refresh(c *gin.Context) {
	var req auth.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokenPair, err := h.usecase.RefreshTokens(c.Request.Context(), &req, c.ClientIP())
	if err != nil {
		h.logger.Error("failed to refresh tokens", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tokenPair)
}

// Logout handles POST /api/v1/auth/logout
func (h *HTTPHandler) Logout(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	deviceID, ok := middleware.GetDeviceID(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id not found in token"})
		return
	}

	if err := h.usecase.Logout(c.Request.Context(), userID, deviceID); err != nil {
		h.logger.Error("failed to logout", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// LogoutAll handles POST /api/v1/auth/logout-all
func (h *HTTPHandler) LogoutAll(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	if err := h.usecase.LogoutAll(c.Request.Context(), userID); err != nil {
		h.logger.Error("failed to logout all sessions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out from all sessions"})
}

// GetSessions handles GET /api/v1/auth/sessions
func (h *HTTPHandler) GetSessions(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	sessions, err := h.usecase.GetActiveSessions(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get sessions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// RevokeSession handles DELETE /api/v1/auth/sessions/:deviceId
func (h *HTTPHandler) RevokeSession(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
		return
	}

	if err := h.usecase.RevokeSession(c.Request.Context(), userID, deviceID); err != nil {
		h.logger.Error("failed to revoke session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "session revoked successfully"})
}
