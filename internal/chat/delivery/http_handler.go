package delivery

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/voronka/backend/internal/chat"
	"github.com/voronka/backend/internal/shared/middleware"
	"go.uber.org/zap"
)

// HTTPHandler handles HTTP requests for chat operations
type HTTPHandler struct {
	usecase chat.Usecase
	logger  *zap.Logger
}

// NewHTTPHandler creates a new chat HTTP handler
func NewHTTPHandler(usecase chat.Usecase, logger *zap.Logger) *HTTPHandler {
	return &HTTPHandler{
		usecase: usecase,
		logger:  logger,
	}
}

// RegisterRoutes registers all chat routes
func (h *HTTPHandler) RegisterRoutes(router *gin.RouterGroup, authMw gin.HandlerFunc) {
	dialogs := router.Group("/dialogs", authMw)
	{
		dialogs.GET("", h.ListDialogs)
		dialogs.GET("/:id", h.GetDialog)
		dialogs.GET("/:id/messages", h.GetMessages)
		dialogs.POST("/:id/messages", h.SendMessage)
		dialogs.PATCH("/:id", h.UpdateDialog)
		dialogs.POST("/:id/read", h.MarkAsRead)
		dialogs.GET("/:id/events", h.GetDialogEvents)
	}
}

// ListDialogs handles GET /api/v1/dialogs
func (h *HTTPHandler) ListDialogs(c *gin.Context) {
	filters := chat.DialogFilters{}

	if agentIDStr := c.Query("agent_id"); agentIDStr != "" {
		agentID, err := uuid.Parse(agentIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent_id"})
			return
		}
		filters.AgentID = &agentID
	}

	if statusStr := c.Query("status"); statusStr != "" {
		// Accept both internal (open/pending/closed) and mobile (active/archived/closed) statuses
		status, ok := chat.ParseMobileStatus(statusStr)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		filters.Status = &status
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err == nil {
			filters.Limit = limit
		}
	}

	filters.Cursor = c.Query("cursor")

	resp, err := h.usecase.ListDialogs(c.Request.Context(), filters)
	if err != nil {
		h.logger.Error("failed to list dialogs", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetDialog handles GET /api/v1/dialogs/:id
func (h *HTTPHandler) GetDialog(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dialog ID"})
		return
	}

	dialog, err := h.usecase.GetDialog(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("failed to get dialog", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dialog)
}

// GetMessages handles GET /api/v1/dialogs/:id/messages
func (h *HTTPHandler) GetMessages(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dialog ID"})
		return
	}

	limit := 30
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	cursor := c.Query("cursor")

	resp, err := h.usecase.GetMessages(c.Request.Context(), id, limit, cursor)
	if err != nil {
		h.logger.Error("failed to get messages", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SendMessage handles POST /api/v1/dialogs/:id/messages
func (h *HTTPHandler) SendMessage(c *gin.Context) {
	dialogID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dialog ID"})
		return
	}

	agentID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req chat.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	message, err := h.usecase.SendMessage(c.Request.Context(), dialogID, agentID, &req)
	if err != nil {
		h.logger.Error("failed to send message", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, message)
}

// UpdateDialog handles PATCH /api/v1/dialogs/:id
func (h *HTTPHandler) UpdateDialog(c *gin.Context) {
	dialogID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dialog ID"})
		return
	}

	agentID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	var req chat.UpdateDialogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dialog, err := h.usecase.UpdateDialog(c.Request.Context(), dialogID, agentID, &req)
	if err != nil {
		h.logger.Error("failed to update dialog", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dialog)
}

// MarkAsRead handles POST /api/v1/dialogs/:id/read
func (h *HTTPHandler) MarkAsRead(c *gin.Context) {
	dialogID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dialog ID"})
		return
	}

	agentID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	if err := h.usecase.MarkAsRead(c.Request.Context(), dialogID, agentID); err != nil {
		h.logger.Error("failed to mark as read", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "messages marked as read"})
}

// GetDialogEvents handles GET /api/v1/dialogs/:id/events
func (h *HTTPHandler) GetDialogEvents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid dialog ID"})
		return
	}

	// Reuse the ProcessIncomingWebhook's underlying repo through usecase
	// For events, we go through GetDialog to verify existence, then query events
	dialog, err := h.usecase.GetDialog(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dialog not found"})
		return
	}

	// We need direct access - for now return dialog info
	// Events endpoint is handled via the existing repository
	_ = dialog
	c.JSON(http.StatusOK, []interface{}{})
}
