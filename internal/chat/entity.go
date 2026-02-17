package chat

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNoChannel is returned when no active channel exists for a platform.
var ErrNoChannel = errors.New("no active channel for platform")

// Channel represents a messaging platform connection
type Channel struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	Platform    Platform        `json:"platform" db:"platform"`
	Credentials json.RawMessage `json:"credentials" db:"credentials"`
	IsActive    bool            `json:"is_active" db:"is_active"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
}

// Platform represents supported messaging platforms
type Platform string

const (
	PlatformTelegram  Platform = "telegram"
	PlatformWhatsApp  Platform = "whatsapp"
	PlatformInstagram Platform = "instagram"
	PlatformFacebook  Platform = "facebook"
)

// Contact represents a customer/lead in the system
type Contact struct {
	ID         uuid.UUID `json:"id" db:"id"`
	ExternalID string    `json:"external_id" db:"external_id"`
	Name       string    `json:"name" db:"name"`
	Phone      string    `json:"phone" db:"phone"`
	Email      string    `json:"email" db:"email"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// Dialog represents a conversation with a contact
type Dialog struct {
	ID             uuid.UUID       `json:"id" db:"id"`
	ChannelID      uuid.UUID       `json:"channel_id" db:"channel_id"`
	ContactID      uuid.UUID       `json:"contact_id" db:"contact_id"`
	CurrentAgentID *uuid.UUID      `json:"current_agent_id,omitempty" db:"current_agent_id"`
	SourceAdID     *uuid.UUID      `json:"source_ad_id,omitempty" db:"source_ad_id"`
	Status         DialogStatus    `json:"status" db:"status"`
	Tags           json.RawMessage `json:"tags" db:"tags"`
	LastMessageAt  time.Time       `json:"last_message_at" db:"last_message_at"`
}

// DialogStatus represents the current state of a dialog
type DialogStatus string

const (
	DialogStatusOpen    DialogStatus = "open"
	DialogStatusPending DialogStatus = "pending"
	DialogStatusClosed  DialogStatus = "closed"
)

// IsValid checks if the dialog status is valid
func (s DialogStatus) IsValid() bool {
	switch s {
	case DialogStatusOpen, DialogStatusPending, DialogStatusClosed:
		return true
	default:
		return false
	}
}

// Message represents a single message in a dialog
type Message struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	DialogID   uuid.UUID       `json:"dialog_id" db:"dialog_id"`
	SenderType SenderType      `json:"sender_type" db:"sender_type"`
	ExternalID string          `json:"external_id,omitempty" db:"external_id"`
	Content    string          `json:"content" db:"content"`
	Payload    json.RawMessage `json:"payload" db:"payload"`
	Metadata   json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// SenderType represents who sent the message
type SenderType string

const (
	SenderTypeCustomer SenderType = "customer"
	SenderTypeAgent    SenderType = "agent"
	SenderTypeAI       SenderType = "ai"
	SenderTypeSystem   SenderType = "system"
)

// DialogEvent represents an audit log entry for dialog changes
type DialogEvent struct {
	ID        uuid.UUID       `json:"id" db:"id"`
	DialogID  uuid.UUID       `json:"dialog_id" db:"dialog_id"`
	EventType EventType       `json:"event_type" db:"event_type"`
	ActorID   *uuid.UUID      `json:"actor_id,omitempty" db:"actor_id"`
	Payload   json.RawMessage `json:"payload" db:"payload"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
}

// EventType represents the type of dialog event
type EventType string

const (
	EventTypeCreated            EventType = "created"
	EventTypeMessageReceived    EventType = "message_received"
	EventTypeAssigned           EventType = "assigned"
	EventTypeTransferred        EventType = "transferred"
	EventTypeStatusChanged      EventType = "status_changed"
	EventTypeAIDraftGenerated   EventType = "ai_draft_generated"
	EventTypePriorityEscalated  EventType = "priority_escalated"
	EventTypeClosed             EventType = "closed"
)

// DialogWithDetails represents a dialog with full details
type DialogWithDetails struct {
	Dialog
	Channel *Channel `json:"channel"`
	Contact *Contact `json:"contact"`
}

// CreateMessageRequest represents the request to send a message
type CreateMessageRequest struct {
	Content    string          `json:"content" binding:"required"`
	Payload    json.RawMessage `json:"payload"`
}

// AssignDialogRequest represents the request to assign a dialog to an agent
type AssignDialogRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
}

// TransferDialogRequest represents the request to transfer a dialog
type TransferDialogRequest struct {
	ToAgentID string `json:"to_agent_id" binding:"required"`
	Reason    string `json:"reason"`
}

// ProcessWebhookRequest is the input for the webhook processing usecase.
type ProcessWebhookRequest struct {
	Platform   Platform
	RawPayload json.RawMessage
	ReceivedAt time.Time
}

// ProcessWebhookResponse contains stats about processed webhook.
type ProcessWebhookResponse struct {
	MessagesCreated int
}