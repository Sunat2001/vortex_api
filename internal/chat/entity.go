package chat

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNoChannel is returned when no active channel exists for a platform.
var ErrNoChannel = errors.New("no active channel for platform")

// ErrMessageAlreadyExists is returned when a message with the same external_id already exists.
var ErrMessageAlreadyExists = errors.New("message with this external_id already exists")

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

// MessageStatus represents the delivery status of a message
type MessageStatus string

const (
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
	MessageStatusFailed    MessageStatus = "failed"
)

// IsValid checks if the message status is valid
func (s MessageStatus) IsValid() bool {
	switch s {
	case MessageStatusSent, MessageStatusDelivered, MessageStatusRead, MessageStatusFailed:
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
	Status     MessageStatus   `json:"status" db:"status"`
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
	EventTypeCreated           EventType = "created"
	EventTypeMessageReceived   EventType = "message_received"
	EventTypeAssigned          EventType = "assigned"
	EventTypeTransferred       EventType = "transferred"
	EventTypeStatusChanged     EventType = "status_changed"
	EventTypeAIDraftGenerated  EventType = "ai_draft_generated"
	EventTypePriorityEscalated EventType = "priority_escalated"
	EventTypeClosed            EventType = "closed"
	EventTypeStatusUpdated     EventType = "status_updated"
)

// DialogWithDetails represents a dialog with full details
type DialogWithDetails struct {
	Dialog
	Channel *Channel `json:"channel"`
	Contact *Contact `json:"contact"`
}

// CreateMessageRequest represents the request to send a message
type CreateMessageRequest struct {
	Content string          `json:"content" binding:"required"`
	Payload json.RawMessage `json:"payload"`
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

// UpdateDialogRequest represents a request to update dialog fields
type UpdateDialogRequest struct {
	Status  *DialogStatus `json:"status"`
	AgentID *string       `json:"agent_id"`
}

// SenderTypeClient is the mobile-facing alias for SenderTypeCustomer
const SenderTypeClient SenderType = "client"

// MapSenderType converts internal sender_type to mobile-facing value
func MapSenderType(st SenderType) SenderType {
	if st == SenderTypeCustomer {
		return SenderTypeClient
	}
	return st
}

// MobileDialogStatus represents mobile-facing dialog statuses
type MobileDialogStatus string

const (
	MobileDialogStatusActive   MobileDialogStatus = "active"
	MobileDialogStatusArchived MobileDialogStatus = "archived"
	MobileDialogStatusClosed   MobileDialogStatus = "closed"
)

// MapDialogStatus converts internal status to mobile-facing value
func MapDialogStatus(s DialogStatus) MobileDialogStatus {
	switch s {
	case DialogStatusOpen:
		return MobileDialogStatusActive
	case DialogStatusPending:
		return MobileDialogStatusArchived
	case DialogStatusClosed:
		return MobileDialogStatusClosed
	default:
		return MobileDialogStatus(s)
	}
}

// ParseMobileStatus converts mobile status back to internal status for filtering
func ParseMobileStatus(s string) (DialogStatus, bool) {
	switch s {
	case "active":
		return DialogStatusOpen, true
	case "archived":
		return DialogStatusPending, true
	case "closed":
		return DialogStatusClosed, true
	case "open", "pending":
		return DialogStatus(s), true
	default:
		return "", false
	}
}

// DialogListItem is the internal struct for dialog list queries (joins contact/channel/agent)
type DialogListItem struct {
	Dialog
	Contact        *Contact `json:"-"`
	Channel        *Channel `json:"-"`
	LastMessage    *Message `json:"-"`
	AgentName      string   `json:"-"`
	UnreadCount    int      `json:"-"`
}

// DialogListItemDTO is the flattened response sent to mobile clients
type DialogListItemDTO struct {
	ID               uuid.UUID          `json:"id"`
	Status           MobileDialogStatus `json:"status"`
	ClientName       string             `json:"client_name"`
	ClientAvatar     *string            `json:"client_avatar"`
	Channel          string             `json:"channel"`
	LastMessage      string             `json:"last_message"`
	LastMessageAt    time.Time          `json:"last_message_at"`
	UnreadCount      int                `json:"unread_count"`
	IsOnline         bool               `json:"is_online"`
	AssignedAgentID  *uuid.UUID         `json:"assigned_agent_id,omitempty"`
	AssignedAgentName string            `json:"assigned_agent_name,omitempty"`
	Tags             json.RawMessage    `json:"tags"`
	ContactID        uuid.UUID          `json:"contact_id"`
	ChannelID        uuid.UUID          `json:"channel_id"`
}

// ToDTO converts a DialogListItem to its mobile-friendly DTO
func (d *DialogListItem) ToDTO() DialogListItemDTO {
	dto := DialogListItemDTO{
		ID:              d.ID,
		Status:          MapDialogStatus(d.Status),
		LastMessageAt:   d.LastMessageAt,
		UnreadCount:     d.UnreadCount,
		IsOnline:        false,
		AssignedAgentID: d.CurrentAgentID,
		AssignedAgentName: d.AgentName,
		Tags:            d.Tags,
		ContactID:       d.ContactID,
		ChannelID:       d.ChannelID,
	}

	if d.Contact != nil {
		dto.ClientName = d.Contact.Name
	}
	if d.Channel != nil {
		dto.Channel = string(d.Channel.Platform)
	}
	if d.LastMessage != nil {
		if d.LastMessage.Content != "" {
			dto.LastMessage = d.LastMessage.Content
		} else {
			dto.LastMessage = mediaTypeLabel(d.LastMessage.Metadata)
		}
	}

	return dto
}

// mediaTypeLabel returns a human-readable label based on media_type in metadata.
// Used as a fallback when Content is empty (e.g. audio, image without caption).
func mediaTypeLabel(metadata json.RawMessage) string {
	if metadata == nil {
		return ""
	}
	var meta struct {
		MediaType string `json:"media_type"`
	}
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return ""
	}
	switch meta.MediaType {
	case "image":
		return "Photo"
	case "video":
		return "Video"
	case "audio":
		return "Voice message"
	case "document":
		return "Document"
	case "location":
		return "Location"
	default:
		return ""
	}
}

// MessageWithSender is a Message with the resolved sender name from joins
type MessageWithSender struct {
	Message
	SenderName string `json:"-"`
}

// MessageDTO is the response shape for messages sent to mobile clients
type MessageDTO struct {
	ID         uuid.UUID       `json:"id"`
	DialogID   uuid.UUID       `json:"dialog_id"`
	SenderType SenderType      `json:"sender_type"`
	SenderName string          `json:"sender_name"`
	ExternalID string          `json:"external_id,omitempty"`
	Content    string          `json:"content"`
	Status     string          `json:"status"`
	Payload    json.RawMessage `json:"payload"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ToDTO converts a Message to its mobile-friendly DTO
func (m *Message) ToDTO(senderName string) MessageDTO {
	return MessageDTO{
		ID:         m.ID,
		DialogID:   m.DialogID,
		SenderType: MapSenderType(m.SenderType),
		SenderName: senderName,
		ExternalID: m.ExternalID,
		Content:    m.Content,
		Status:     string(m.Status),
		Payload:    m.Payload,
		Metadata:   m.Metadata,
		CreatedAt:  m.CreatedAt,
	}
}

// MessageListResponse represents a cursor-paginated list of messages
type MessageListResponse struct {
	Messages   []MessageDTO `json:"messages"`
	NextCursor string       `json:"next_cursor,omitempty"`
	HasMore    bool         `json:"has_more"`
}

// DialogListResponse represents a paginated list of dialogs
type DialogListResponse struct {
	Dialogs    []DialogListItemDTO `json:"dialogs"`
	NextCursor string              `json:"next_cursor,omitempty"`
	HasMore    bool                `json:"has_more"`
}

// NewMessageEvent is the domain event emitted when a new inbound message is persisted.
type NewMessageEvent struct {
	Message *Message
	Dialog  *Dialog
	Contact *Contact
	AgentID *uuid.UUID // nil if unassigned — broadcast to all agents
}

// DialogUpdatedEvent is emitted when dialog status or assignment changes.
type DialogUpdatedEvent struct {
	DialogID  uuid.UUID
	AgentID   *uuid.UUID
	EventType EventType
}

// MessageStatusEvent is emitted when a delivery receipt changes message status.
type MessageStatusEvent struct {
	MessageID uuid.UUID
	DialogID  uuid.UUID
	Status    MessageStatus
	AgentID   *uuid.UUID
}

// ProcessWebhookRequest is the input for the webhook processing usecase.
type ProcessWebhookRequest struct {
	Platform   Platform
	RawPayload json.RawMessage
	ReceivedAt time.Time
}

// ProcessWebhookResponse contains stats about processed webhook.
type ProcessWebhookResponse struct {
	MessagesCreated   int
	StatusesProcessed int
}

// ParsePlatform maps a source string to a Platform constant.
func ParsePlatform(source string) Platform {
	switch strings.ToLower(source) {
	case "facebook":
		return PlatformFacebook
	case "instagram":
		return PlatformInstagram
	case "whatsapp":
		return PlatformWhatsApp
	case "telegram":
		return PlatformTelegram
	default:
		return Platform(source)
	}
}
