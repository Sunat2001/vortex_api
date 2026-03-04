package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/voronka/backend/internal/chat/parser"
)

// Usecase defines the business logic interface for the chat domain.
type Usecase interface {
	ProcessIncomingWebhook(ctx context.Context, req *ProcessWebhookRequest) (*ProcessWebhookResponse, error)
}

type usecaseImpl struct {
	repo   Repository
	logger *zap.Logger
}

// NewUsecase creates a new chat usecase.
func NewUsecase(repo Repository, logger *zap.Logger) Usecase {
	return &usecaseImpl{
		repo:   repo,
		logger: logger,
	}
}

// ProcessIncomingWebhook parses the platform-specific webhook and persists
// contacts, dialogs, messages, and events.
func (u *usecaseImpl) ProcessIncomingWebhook(ctx context.Context, req *ProcessWebhookRequest) (*ProcessWebhookResponse, error) {
	// 1. Get the appropriate parser for the platform.
	p, err := parser.New(string(req.Platform))
	if err != nil {
		return nil, fmt.Errorf("unsupported platform %s: %w", req.Platform, err)
	}

	// 2. Parse webhook into a list of events.
	u.logger.Debug("raw webhook payload",
		zap.String("platform", string(req.Platform)),
		zap.String("payload", string(req.RawPayload)),
	)

	events, err := p.Parse(req.RawPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	u.logger.Debug("parsed webhook events",
		zap.String("platform", string(req.Platform)),
		zap.Int("count", len(events)),
	)

	// 3. Resolve channel for this platform (must be pre-configured).
	channel, err := u.repo.GetChannelByPlatform(ctx, req.Platform)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrNoChannel, req.Platform, err)
	}

	// 4. Process each event.
	resp := &ProcessWebhookResponse{}
	for _, event := range events {
		switch event.Kind {
		case parser.EventKindMessage:
			if err := u.processMessage(ctx, channel, event.Message); err != nil {
				u.logger.Error("failed to process incoming message",
					zap.String("platform", string(req.Platform)),
					zap.String("sender", event.Message.ExternalSenderID),
					zap.Error(err),
				)
				continue
			}
			resp.MessagesCreated++

		case parser.EventKindStatus:
			if err := u.processStatusUpdate(ctx, channel, event.Status); err != nil {
				u.logger.Error("failed to process status update",
					zap.String("platform", string(req.Platform)),
					zap.String("external_message_id", event.Status.ExternalMessageID),
					zap.Error(err),
				)
				continue
			}
			resp.StatusesProcessed++
		}
	}

	return resp, nil
}

// processMessage handles a single incoming message: upsert contact, ensure
// dialog, create message, update dialog, create event.
func (u *usecaseImpl) processMessage(ctx context.Context, channel *Channel, msg *parser.IncomingMessage) error {
	// 1. Upsert contact.
	contact := &Contact{
		ID:         uuid.New(),
		ExternalID: msg.ExternalSenderID,
		Name:       msg.ContactName,
		Phone:      msg.ContactPhone,
		CreatedAt:  time.Now(),
	}
	if err := u.repo.UpsertContact(ctx, contact); err != nil {
		return fmt.Errorf("failed to upsert contact: %w", err)
	}

	// 2. Get or create dialog.
	dialog, err := u.repo.GetOrCreateDialog(ctx, channel.ID, contact.ID)
	if err != nil {
		return fmt.Errorf("failed to get or create dialog: %w", err)
	}

	// 3. Create message.
	payload := msg.Payload
	if payload == nil {
		payload = json.RawMessage(`{}`)
	}

	metadataMap := map[string]interface{}{
		"media_type": string(msg.MediaType),
	}

	// Merge parser-level metadata (errors, from_logical_id, country_code, etc.).
	if msg.Metadata != nil {
		var parserMeta map[string]interface{}
		if err := json.Unmarshal(msg.Metadata, &parserMeta); err == nil {
			for k, v := range parserMeta {
				metadataMap[k] = v
			}
		}
	}

	metadataJSON, _ := json.Marshal(metadataMap)

	createdAt := msg.Timestamp
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	message := &Message{
		ID:         uuid.New(),
		DialogID:   dialog.ID,
		SenderType: SenderTypeCustomer,
		ExternalID: msg.ExternalMessageID,
		Content:    msg.Content,
		Payload:    payload,
		Metadata:   json.RawMessage(metadataJSON),
		CreatedAt:  createdAt,
	}
	if err := u.repo.CreateMessage(ctx, message); err != nil {
		if errors.Is(err, ErrMessageAlreadyExists) {
			u.logger.Info("message already exists, skipping duplicate",
				zap.String("external_id", message.ExternalID),
				zap.String("dialog_id", dialog.ID.String()),
			)
			return nil
		}
		return fmt.Errorf("failed to create message: %w", err)
	}

	u.logger.Info("message created",
		zap.String("message_id", message.ID.String()),
		zap.String("dialog_id", dialog.ID.String()),
	)

	// 4. Update dialog timestamp (non-fatal).
	if err := u.repo.UpdateDialogLastMessageAt(ctx, dialog.ID); err != nil {
		u.logger.Warn("failed to update dialog last_message_at", zap.Error(err))
	}

	// 5. Create dialog event (non-fatal).
	event := &DialogEvent{
		ID:        uuid.New(),
		DialogID:  dialog.ID,
		EventType: EventTypeMessageReceived,
		Payload:   json.RawMessage(fmt.Sprintf(`{"message_id":"%s"}`, message.ID.String())),
		CreatedAt: time.Now(),
	}
	if err := u.repo.CreateDialogEvent(ctx, event); err != nil {
		u.logger.Warn("failed to create dialog event", zap.Error(err))
	}

	return nil
}

// processStatusUpdate handles a delivery/read receipt: looks up the original
// message by external_id and creates a dialog_event with type "status_updated".
func (u *usecaseImpl) processStatusUpdate(ctx context.Context, channel *Channel, status *parser.StatusUpdate) error {
	// 1. Find the original message.
	message, err := u.repo.GetMessageByExternalID(ctx, status.ExternalMessageID)
	if err != nil {
		return fmt.Errorf("failed to find message for status update (external_id=%s): %w", status.ExternalMessageID, err)
	}

	// 2. Create a dialog event.
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"message_id":          message.ID.String(),
		"external_message_id": status.ExternalMessageID,
		"status":              status.Status,
		"timestamp":           status.Timestamp.Format(time.RFC3339),
	})

	event := &DialogEvent{
		ID:        uuid.New(),
		DialogID:  message.DialogID,
		EventType: EventTypeStatusUpdated,
		Payload:   json.RawMessage(eventPayload),
		CreatedAt: time.Now(),
	}

	if err := u.repo.CreateDialogEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to create status update event: %w", err)
	}

	u.logger.Info("status update processed",
		zap.String("message_id", message.ID.String()),
		zap.String("status", status.Status),
	)

	return nil
}
