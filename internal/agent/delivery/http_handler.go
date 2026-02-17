package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/voronka/backend/internal/agent"
	"go.uber.org/zap"
)

// HTTPHandler handles HTTP requests for agent operations
type HTTPHandler struct {
	usecase agent.Usecase
	logger  *zap.Logger
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(usecase agent.Usecase, logger *zap.Logger) *HTTPHandler {
	return &HTTPHandler{
		usecase: usecase,
		logger:  logger,
	}
}

// RegisterRoutes registers all agent routes
func (h *HTTPHandler) RegisterRoutes(router *gin.RouterGroup) {
	agents := router.Group("/agents")
	{
		agents.POST("/invite", h.InviteAgent)
		agents.GET("", h.ListAgents)
		agents.GET("/workload", h.GetAgentWorkload)
		agents.PATCH("/me/status", h.UpdateMyStatus)
		agents.GET("/:id", h.GetAgent)
	}

	roles := router.Group("/roles")
	{
		roles.GET("", h.ListRoles)
		roles.POST("", h.CreateRole)
		roles.GET("/:id", h.GetRole)
		roles.DELETE("/:id", h.DeleteRole)
		roles.POST("/:id/permissions", h.AssignPermissionsToRole)
	}

	permissions := router.Group("/permissions")
	{
		permissions.GET("", h.ListPermissions)
	}
}

// InviteAgent handles POST /agents/invite
func (h *HTTPHandler) InviteAgent(c *gin.Context) {
	var req agent.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.usecase.InviteAgent(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to invite agent", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// ListAgents handles GET /agents
func (h *HTTPHandler) ListAgents(c *gin.Context) {
	var status *agent.UserStatus
	if statusStr := c.Query("status"); statusStr != "" {
		s := agent.UserStatus(statusStr)
		if !s.IsValid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		status = &s
	}

	users, err := h.usecase.ListAgents(c.Request.Context(), status)
	if err != nil {
		h.logger.Error("failed to list agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, users)
}

// GetAgent handles GET /agents/:id
func (h *HTTPHandler) GetAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}

	user, err := h.usecase.GetAgent(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("failed to get agent", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateMyStatus handles PATCH /agents/me/status
func (h *HTTPHandler) UpdateMyStatus(c *gin.Context) {
	var req agent.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Get user ID from JWT token in context
	// For now, we'll expect it to be passed in the request
	userID := uuid.Nil // Replace with actual user ID from JWT

	if err := h.usecase.UpdateAgentStatus(c.Request.Context(), userID, req.Status); err != nil {
		h.logger.Error("failed to update agent status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "status updated successfully"})
}

// GetAgentWorkload handles GET /agents/workload
func (h *HTTPHandler) GetAgentWorkload(c *gin.Context) {
	workload, err := h.usecase.GetAgentWorkload(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to get agent workload", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, workload)
}

// CreateRole handles POST /roles
func (h *HTTPHandler) CreateRole(c *gin.Context) {
	var req agent.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role, err := h.usecase.CreateRole(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to create role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, role)
}

// ListRoles handles GET /roles
func (h *HTTPHandler) ListRoles(c *gin.Context) {
	roles, err := h.usecase.ListRoles(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to list roles", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// GetRole handles GET /roles/:id
func (h *HTTPHandler) GetRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role ID"})
		return
	}

	role, err := h.usecase.GetRole(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("failed to get role", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, role)
}

// DeleteRole handles DELETE /roles/:id
func (h *HTTPHandler) DeleteRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role ID"})
		return
	}

	if err := h.usecase.DeleteRole(c.Request.Context(), id); err != nil {
		h.logger.Error("failed to delete role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "role deleted successfully"})
}

// AssignPermissionsToRole handles POST /roles/:id/permissions
func (h *HTTPHandler) AssignPermissionsToRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role ID"})
		return
	}

	var req agent.AssignPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse permission IDs
	permissionIDs := make([]uuid.UUID, 0, len(req.PermissionIDs))
	for _, pidStr := range req.PermissionIDs {
		pid, err := uuid.Parse(pidStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid permission ID: " + pidStr})
			return
		}
		permissionIDs = append(permissionIDs, pid)
	}

	if err := h.usecase.AssignPermissionsToRole(c.Request.Context(), id, permissionIDs); err != nil {
		h.logger.Error("failed to assign permissions to role", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "permissions assigned successfully"})
}

// ListPermissions handles GET /permissions
func (h *HTTPHandler) ListPermissions(c *gin.Context) {
	var module *string
	if m := c.Query("module"); m != "" {
		module = &m
	}

	permissions, err := h.usecase.ListPermissions(c.Request.Context(), module)
	if err != nil {
		h.logger.Error("failed to list permissions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, permissions)
}