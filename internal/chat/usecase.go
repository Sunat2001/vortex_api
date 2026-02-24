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

	// 2. Parse webhook into a list of incoming messages.
	u.logger.Debug("raw webhook payload",
		zap.String("platform", string(req.Platform)),
		zap.String("payload", string(req.RawPayload)),
	)

	incoming, err := p.Parse(req.RawPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	u.logger.Debug("parsed webhook messages",
		zap.String("platform", string(req.Platform)),
		zap.Int("count", len(incoming)),
	)

	// 3. Resolve channel for this platform (must be pre-configured).
	channel, err := u.repo.GetChannelByPlatform(ctx, req.Platform)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %w", ErrNoChannel, req.Platform, err)
	}

	// 4. Process each message.
	resp := &ProcessWebhookResponse{}
	for _, msg := range incoming {
		if err := u.processMessage(ctx, channel, msg); err != nil {
			u.logger.Error("failed to process incoming message",
				zap.String("platform", string(req.Platform)),
				zap.String("sender", msg.ExternalSenderID),
				zap.Error(err),
			)
			continue
		}
		resp.MessagesCreated++
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

	metadataJSON, _ := json.Marshal(map[string]interface{}{
		"platform":   channel.Platform,
		"media_type": string(msg.MediaType),
	})

	message := &Message{
		ID:         uuid.New(),
		DialogID:   dialog.ID,
		SenderType: SenderTypeCustomer,
		ExternalID: msg.ExternalMessageID,
		Content:    msg.Content,
		Payload:    payload,
		Metadata:   json.RawMessage(metadataJSON),
		CreatedAt:  time.Now(),
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
