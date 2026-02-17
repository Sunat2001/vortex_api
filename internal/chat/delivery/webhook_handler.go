package delivery

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voronka/backend/internal/chat"
	"github.com/voronka/backend/internal/shared/config"
	"github.com/voronka/backend/internal/shared/redis"
	"go.uber.org/zap"
)

// WebhookHandler handles webhook ingress from Meta platforms (Facebook, Instagram, WhatsApp)
// Implements the "Validate -> Enqueue -> Respond" pattern with NO business logic
type WebhookHandler struct {
	streamManager *redis.StreamManager
	config        *config.WebhooksConfig
	logger        *zap.Logger
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(streamManager *redis.StreamManager, cfg *config.WebhooksConfig, logger *zap.Logger) *WebhookHandler {
	return &WebhookHandler{
		streamManager: streamManager,
		config:        cfg,
		logger:        logger,
	}
}

// RegisterRoutes registers all webhook routes with complete platform isolation
func (h *WebhookHandler) RegisterRoutes(router *gin.RouterGroup) {
	webhooks := router.Group("/webhooks")
	{
		// Facebook webhook endpoints
		webhooks.GET("/facebook", h.VerifyFacebookWebhook)
		webhooks.POST("/facebook", h.ValidateSignature(string(chat.PlatformFacebook)), h.HandleFacebookWebhook)

		// Instagram webhook endpoints
		webhooks.GET("/instagram", h.VerifyInstagramWebhook)
		webhooks.POST("/instagram", h.ValidateSignature(string(chat.PlatformInstagram)), h.HandleInstagramWebhook)

		// WhatsApp webhook endpoints
		webhooks.GET("/whatsapp", h.VerifyWhatsAppWebhook)
		webhooks.POST("/whatsapp", h.ValidateSignature(string(chat.PlatformWhatsApp)), h.HandleWhatsAppWebhook)
	}
}

// VerifyFacebookWebhook handles GET /webhooks/facebook (Meta verification)
func (h *WebhookHandler) VerifyFacebookWebhook(c *gin.Context) {
	h.verifyWebhook(c, h.config.Facebook.VerifyToken, string(chat.PlatformFacebook))
}

// VerifyInstagramWebhook handles GET /webhooks/instagram (Meta verification)
func (h *WebhookHandler) VerifyInstagramWebhook(c *gin.Context) {
	h.verifyWebhook(c, h.config.Instagram.VerifyToken, string(chat.PlatformInstagram))
}

// VerifyWhatsAppWebhook handles GET /webhooks/whatsapp (Meta verification)
func (h *WebhookHandler) VerifyWhatsAppWebhook(c *gin.Context) {
	h.verifyWebhook(c, h.config.WhatsApp.VerifyToken, string(chat.PlatformWhatsApp))
}

// verifyWebhook implements standard Meta webhook verification
// Checks hub.mode, hub.verify_token, and returns hub.challenge
func (h *WebhookHandler) verifyWebhook(c *gin.Context, expectedToken, platform string) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	h.logger.Info("webhook verification request",
		zap.String("platform", platform),
		zap.String("mode", mode),
	)

	if mode == "subscribe" && token == expectedToken {
		h.logger.Info("webhook verified successfully",
			zap.String("platform", platform),
		)
		c.String(http.StatusOK, challenge)
		return
	}

	h.logger.Warn("webhook verification failed",
		zap.String("platform", platform),
		zap.String("mode", mode),
		zap.Bool("token_match", token == expectedToken),
	)
	c.JSON(http.StatusForbidden, gin.H{"error": "verification failed"})
}

// ValidateSignature is a reusable Gin middleware that validates X-Hub-Signature-256
// using HMAC-SHA256 with the platform-specific app secret
func (h *WebhookHandler) ValidateSignature(platform string) gin.HandlerFunc {
	return func(c *gin.Context) {
		signature := c.GetHeader("X-Hub-Signature-256")
		if signature == "" {
			h.logger.Warn("missing signature header",
				zap.String("platform", platform),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing signature"})
			return
		}

		// Read body once and store for both validation and handler use
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			h.logger.Error("failed to read request body",
				zap.String("platform", platform),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to read body"})
			return
		}

		h.logger.Debug("webhook raw body",
			zap.String("platform", platform),
			zap.String("body", string(bodyBytes)),
		)

		// Restore body for downstream handler
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Get platform-specific secret
		secret := h.getAppSecret(platform)
		if secret == "" {
			h.logger.Error("missing app secret for platform",
				zap.String("platform", platform),
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "server configuration error"})
			return
		}

		// Compute HMAC-SHA256
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(bodyBytes)
		expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		// Constant-time comparison to prevent timing attacks
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			h.logger.Warn("signature validation failed",
				zap.String("platform", platform),
				zap.String("received_signature", signature),
				zap.String("expected_signature", expectedSignature),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}

		// Store body bytes in context for handler
		c.Set("webhook_body", bodyBytes)
		c.Next()
	}
}

// HandleFacebookWebhook handles POST /webhooks/facebook
func (h *WebhookHandler) HandleFacebookWebhook(c *gin.Context) {
	h.handleWebhook(c, string(chat.PlatformFacebook))
}

// HandleInstagramWebhook handles POST /webhooks/instagram
func (h *WebhookHandler) HandleInstagramWebhook(c *gin.Context) {
	h.handleWebhook(c, string(chat.PlatformInstagram))
}

// HandleWhatsAppWebhook handles POST /webhooks/whatsapp
func (h *WebhookHandler) HandleWhatsAppWebhook(c *gin.Context) {
	h.handleWebhook(c, string(chat.PlatformWhatsApp))
}

// handleWebhook implements the core "Validate -> Enqueue -> Respond" pattern
// NO business logic, NO database operations - only security validation and reliable enqueuing
func (h *WebhookHandler) handleWebhook(c *gin.Context, platform string) {
	// Get body from middleware (already validated)
	bodyBytes, exists := c.Get("webhook_body")
	if !exists {
		h.logger.Error("webhook body not found in context",
			zap.String("platform", platform),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	body, ok := bodyBytes.([]byte)
	if !ok {
		h.logger.Error("invalid webhook body type",
			zap.String("platform", platform),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Prepare stream message with metadata
	timestamp := time.Now().UTC().Format(time.RFC3339)
	streamMessage := map[string]interface{}{
		"source":      platform,
		"received_at": timestamp,
		"payload":     string(body),
	}

	// Push to Redis Stream - MUST succeed before responding
	messageID, err := h.streamManager.AddMessage(c.Request.Context(), redis.StreamInboundRaw, streamMessage)
	if err != nil {
		h.logger.Error("failed to enqueue webhook to redis stream",
			zap.String("platform", platform),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process webhook"})
		return
	}

	h.logger.Info("webhook enqueued successfully",
		zap.String("platform", platform),
		zap.String("message_id", messageID),
		zap.Int("payload_size", len(body)),
	)

	// Return 200 OK immediately after successful enqueue to prevent platform timeout
	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

// getAppSecret returns the platform-specific app secret for signature validation
func (h *WebhookHandler) getAppSecret(platform string) string {
	switch chat.Platform(strings.ToLower(platform)) {
	case chat.PlatformFacebook:
		return h.config.Facebook.AppSecret
	case chat.PlatformInstagram:
		return h.config.Instagram.AppSecret
	case chat.PlatformWhatsApp:
		return h.config.WhatsApp.AppSecret
	default:
		return ""
	}
}
