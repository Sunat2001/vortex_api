package parser

import (
	"encoding/json"
	"fmt"
	"time"
)

// IncomingMessage represents a parsed message extracted from a platform webhook.
type IncomingMessage struct {
	ExternalSenderID  string
	ExternalMessageID string
	Content           string
	MediaType         MediaType
	Payload           json.RawMessage // Only platform-specific data (media URLs, buttons, etc.)
	ContactName       string
	ContactPhone      string
	Timestamp         time.Time
}

// MediaType represents the type of media in a message.
type MediaType string

const (
	MediaTypeText     MediaType = "text"
	MediaTypeImage    MediaType = "image"
	MediaTypeVideo    MediaType = "video"
	MediaTypeAudio    MediaType = "audio"
	MediaTypeDocument MediaType = "document"
)

// WebhookParser extracts incoming messages from a platform-specific webhook payload.
type WebhookParser interface {
	Parse(raw json.RawMessage) ([]*IncomingMessage, error)
}

// New returns the appropriate parser for the given platform string.
func New(platform string) (WebhookParser, error) {
	switch platform {
	case "facebook", "instagram":
		return &MetaParser{}, nil
	case "whatsapp":
		return &WhatsAppParser{}, nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}