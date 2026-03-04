package parser

import (
	"encoding/json"
	"fmt"
	"time"
)

// EventKind discriminates the type of event parsed from a webhook.
type EventKind string

const (
	EventKindMessage EventKind = "message"
	EventKindStatus  EventKind = "status"
)

// IncomingMessage represents a parsed message extracted from a platform webhook.
type IncomingMessage struct {
	ExternalSenderID  string
	ExternalMessageID string
	Content           string
	MediaType         MediaType
	Payload           json.RawMessage // Only platform-specific data (media URLs, buttons, etc.)
	Metadata          json.RawMessage // Platform-specific diagnostic data (errors, logical IDs, etc.)
	ContactName       string
	ContactPhone      string
	Timestamp         time.Time
}

// StatusUpdate represents a delivery/read receipt from the platform.
type StatusUpdate struct {
	ExternalMessageID string
	Status            string // "sent", "delivered", "read", "failed"
	Timestamp         time.Time
}

// WebhookEvent is a discriminated union returned by parsers.
type WebhookEvent struct {
	Kind    EventKind
	Message *IncomingMessage // when Kind == EventKindMessage
	Status  *StatusUpdate    // when Kind == EventKindStatus
}

// MediaType represents the type of media in a message.
type MediaType string

const (
	MediaTypeText        MediaType = "text"
	MediaTypeImage       MediaType = "image"
	MediaTypeVideo       MediaType = "video"
	MediaTypeAudio       MediaType = "audio"
	MediaTypeDocument    MediaType = "document"
	MediaTypeLocation    MediaType = "location"
	MediaTypeUnsupported MediaType = "unsupported"
)

// WebhookParser extracts incoming messages from a platform-specific webhook payload.
type WebhookParser interface {
	Parse(raw json.RawMessage) ([]WebhookEvent, error)
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