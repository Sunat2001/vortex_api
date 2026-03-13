package chat

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the interface for chat-related database operations
type Repository interface {
	// Dialog operations
	CreateDialog(ctx context.Context, dialog *Dialog) error
	GetDialogByID(ctx context.Context, id uuid.UUID) (*Dialog, error)
	GetOrCreateDialog(ctx context.Context, channelID, contactID uuid.UUID) (*Dialog, error) // For webhook processing
	ListDialogs(ctx context.Context, filters DialogFilters) ([]Dialog, error)
	UpdateDialogStatus(ctx context.Context, id uuid.UUID, status DialogStatus) error
	UpdateDialogLastMessageAt(ctx context.Context, dialogID uuid.UUID) error // For webhook processing
	AssignDialogToAgent(ctx context.Context, dialogID, agentID uuid.UUID) error
	TransferDialog(ctx context.Context, dialogID, fromAgentID, toAgentID uuid.UUID) error

	// Message operations
	CreateMessage(ctx context.Context, message *Message) error
	GetMessageByExternalID(ctx context.Context, externalID string) (*Message, error)
	GetMessagesByDialogID(ctx context.Context, dialogID uuid.UUID, limit, offset int) ([]Message, error)
	GetLastMessage(ctx context.Context, dialogID uuid.UUID) (*Message, error)
	UpdateMessageStatus(ctx context.Context, externalID string, status MessageStatus) error

	// Dialog Event operations (Audit Log)
	CreateDialogEvent(ctx context.Context, event *DialogEvent) error
	GetDialogEvents(ctx context.Context, dialogID uuid.UUID) ([]DialogEvent, error)

	// Contact operations
	CreateContact(ctx context.Context, contact *Contact) error
	GetContactByExternalID(ctx context.Context, externalID string) (*Contact, error)
	GetContactByID(ctx context.Context, id uuid.UUID) (*Contact, error)
	UpsertContact(ctx context.Context, contact *Contact) error // For webhook processing

	// Channel operations
	GetChannelByID(ctx context.Context, id uuid.UUID) (*Channel, error)
	GetChannelByPlatform(ctx context.Context, platform Platform) (*Channel, error) // For webhook processing
	ListChannels(ctx context.Context, platform *Platform) ([]Channel, error)

	// Agent-facing operations
	ListDialogsWithDetails(ctx context.Context, filters DialogFilters) ([]DialogListItem, error)
	GetDialogWithContact(ctx context.Context, id uuid.UUID) (*DialogWithDetails, error)
	ListMessagesCursor(ctx context.Context, cursor MessageCursor) ([]MessageWithSender, error)
	CreateAgentMessage(ctx context.Context, message *Message) error
}

// DialogFilters represents filters for listing dialogs
type DialogFilters struct {
	AgentID  *uuid.UUID
	Status   *DialogStatus
	Platform *Platform
	Limit    int
	Cursor   string // last_message_at cursor for pagination
}

// MessageCursor represents cursor-based pagination params for messages
type MessageCursor struct {
	DialogID uuid.UUID
	Limit    int
	Cursor   string // created_at,id cursor
}