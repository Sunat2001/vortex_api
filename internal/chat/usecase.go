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

// Notifier is the domain-level abstraction for pushing real-time events
// to connected clients. The delivery layer implements this interface.
type Notifier interface {
	NotifyNewMessage(ctx context.Context, event *NewMessageEvent)
	NotifyDialogUpdated(ctx context.Context, event *DialogUpdatedEvent)
	NotifyMessageStatusChanged(ctx context.Context, event *MessageStatusEvent)
}

// Usecase defines the business logic interface for the chat domain.
type Usecase interface {
	ProcessIncomingWebhook(ctx context.Context, req *ProcessWebhookRequest) (*ProcessWebhookResponse, error)

	// Agent-facing operations
	ListDialogs(ctx context.Context, filters DialogFilters) (*DialogListResponse, error)
	GetDialog(ctx context.Context, dialogID uuid.UUID) (*DialogWithDetails, error)
	GetMessages(ctx context.Context, dialogID uuid.UUID, limit int, cursor string) (*MessageListResponse, error)
	SendMessage(ctx context.Context, dialogID, agentID uuid.UUID, req *CreateMessageRequest) (*MessageDTO, error)
	UpdateDialog(ctx context.Context, dialogID, agentID uuid.UUID, req *UpdateDialogRequest) (*DialogWithDetails, error)
	MarkAsRead(ctx context.Context, dialogID, agentID uuid.UUID) error
}

type usecaseImpl struct {
	repo     Repository
	notifier Notifier
	logger   *zap.Logger
}

// NewUsecase creates a new chat usecase. notifier may be nil (notifications disabled).
func NewUsecase(repo Repository, notifier Notifier, logger *zap.Logger) Usecase {
	return &usecaseImpl{
		repo:     repo,
		notifier: notifier,
		logger:   logger,
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

	// 6. Notify connected clients (non-fatal, fire-and-forget).
	if u.notifier != nil {
		u.notifier.NotifyNewMessage(ctx, &NewMessageEvent{
			Message: message,
			Dialog:  dialog,
			Contact: contact,
			AgentID: dialog.CurrentAgentID,
		})
	}

	return nil
}

// ListDialogs returns a paginated list of dialogs with flattened mobile-friendly DTOs.
func (u *usecaseImpl) ListDialogs(ctx context.Context, filters DialogFilters) (*DialogListResponse, error) {
	limit := filters.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	filters.Limit = limit

	items, err := u.repo.ListDialogsWithDetails(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list dialogs: %w", err)
	}

	hasMore := false
	if len(items) > limit {
		items = items[:limit]
		hasMore = true
	}

	// Fetch last message for each dialog
	for i := range items {
		lastMsg, err := u.repo.GetLastMessage(ctx, items[i].ID)
		if err == nil {
			items[i].LastMessage = lastMsg
		}
	}

	// Convert to DTOs
	dtos := make([]DialogListItemDTO, len(items))
	for i, item := range items {
		dtos[i] = item.ToDTO()
	}

	resp := &DialogListResponse{
		Dialogs: dtos,
		HasMore: hasMore,
	}

	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		resp.NextCursor = last.LastMessageAt.Format(time.RFC3339Nano)
	}

	if resp.Dialogs == nil {
		resp.Dialogs = []DialogListItemDTO{}
	}

	return resp, nil
}

// GetDialog returns a single dialog with full details.
func (u *usecaseImpl) GetDialog(ctx context.Context, dialogID uuid.UUID) (*DialogWithDetails, error) {
	dialog, err := u.repo.GetDialogWithContact(ctx, dialogID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dialog: %w", err)
	}
	return dialog, nil
}

// GetMessages returns cursor-paginated messages for a dialog with sender names.
func (u *usecaseImpl) GetMessages(ctx context.Context, dialogID uuid.UUID, limit int, cursor string) (*MessageListResponse, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	messagesWithSender, err := u.repo.ListMessagesCursor(ctx, MessageCursor{
		DialogID: dialogID,
		Limit:    limit,
		Cursor:   cursor,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	hasMore := false
	if len(messagesWithSender) > limit {
		messagesWithSender = messagesWithSender[:limit]
		hasMore = true
	}

	// Convert to DTOs
	dtos := make([]MessageDTO, len(messagesWithSender))
	for i, mws := range messagesWithSender {
		dtos[i] = mws.Message.ToDTO(mws.SenderName)
	}

	resp := &MessageListResponse{
		Messages: dtos,
		HasMore:  hasMore,
	}

	if hasMore && len(messagesWithSender) > 0 {
		last := messagesWithSender[len(messagesWithSender)-1]
		resp.NextCursor = fmt.Sprintf("%s,%s", last.CreatedAt.Format(time.RFC3339Nano), last.ID.String())
	}

	if resp.Messages == nil {
		resp.Messages = []MessageDTO{}
	}

	return resp, nil
}

// SendMessage creates an agent message and enqueues it for outbound delivery.
func (u *usecaseImpl) SendMessage(ctx context.Context, dialogID, agentID uuid.UUID, req *CreateMessageRequest) (*MessageDTO, error) {
	// Verify dialog exists
	dialog, err := u.repo.GetDialogByID(ctx, dialogID)
	if err != nil {
		return nil, fmt.Errorf("dialog not found: %w", err)
	}

	payload := req.Payload
	if payload == nil {
		payload = json.RawMessage(`{}`)
	}

	message := &Message{
		ID:         uuid.New(),
		DialogID:   dialog.ID,
		SenderType: SenderTypeAgent,
		Content:    req.Content,
		Payload:    payload,
		Metadata:   json.RawMessage(fmt.Sprintf(`{"agent_id":"%s"}`, agentID.String())),
		CreatedAt:  time.Now(),
	}

	if err := u.repo.CreateAgentMessage(ctx, message); err != nil {
		return nil, fmt.Errorf("failed to create agent message: %w", err)
	}

	// Update dialog timestamp
	if err := u.repo.UpdateDialogLastMessageAt(ctx, dialog.ID); err != nil {
		u.logger.Warn("failed to update dialog last_message_at", zap.Error(err))
	}

	u.logger.Info("agent message created",
		zap.String("message_id", message.ID.String()),
		zap.String("dialog_id", dialog.ID.String()),
		zap.String("agent_id", agentID.String()),
	)

	// Notify connected clients about the new agent message.
	if u.notifier != nil {
		u.notifier.NotifyNewMessage(ctx, &NewMessageEvent{
			Message: message,
			Dialog:  dialog,
			AgentID: dialog.CurrentAgentID,
		})
	}

	// Return as DTO — sender_name will be resolved by the agent's name lookup
	// For now, use empty string as the agent name is not fetched here
	dto := message.ToDTO("") // Agent name can be resolved client-side from the token
	return &dto, nil
}

// UpdateDialog updates dialog status or assignment, returns the updated dialog.
func (u *usecaseImpl) UpdateDialog(ctx context.Context, dialogID, agentID uuid.UUID, req *UpdateDialogRequest) (*DialogWithDetails, error) {
	if req.Status != nil {
		if !req.Status.IsValid() {
			return nil, fmt.Errorf("invalid status: %s", *req.Status)
		}
		if err := u.repo.UpdateDialogStatus(ctx, dialogID, *req.Status); err != nil {
			return nil, fmt.Errorf("failed to update dialog status: %w", err)
		}

		// Create status change event
		eventPayload, _ := json.Marshal(map[string]string{
			"status":   string(*req.Status),
			"actor_id": agentID.String(),
		})
		event := &DialogEvent{
			ID:        uuid.New(),
			DialogID:  dialogID,
			EventType: EventTypeStatusChanged,
			ActorID:   &agentID,
			Payload:   json.RawMessage(eventPayload),
			CreatedAt: time.Now(),
		}
		if err := u.repo.CreateDialogEvent(ctx, event); err != nil {
			u.logger.Warn("failed to create status change event", zap.Error(err))
		}
	}

	if req.AgentID != nil {
		targetAgentID, err := uuid.Parse(*req.AgentID)
		if err != nil {
			return nil, fmt.Errorf("invalid agent_id: %w", err)
		}
		if err := u.repo.AssignDialogToAgent(ctx, dialogID, targetAgentID); err != nil {
			return nil, fmt.Errorf("failed to assign dialog: %w", err)
		}

		eventPayload, _ := json.Marshal(map[string]string{
			"assigned_to": targetAgentID.String(),
			"assigned_by": agentID.String(),
		})
		event := &DialogEvent{
			ID:        uuid.New(),
			DialogID:  dialogID,
			EventType: EventTypeAssigned,
			ActorID:   &agentID,
			Payload:   json.RawMessage(eventPayload),
			CreatedAt: time.Now(),
		}
		if err := u.repo.CreateDialogEvent(ctx, event); err != nil {
			u.logger.Warn("failed to create assignment event", zap.Error(err))
		}
	}

	// Return updated dialog
	dialog, err := u.repo.GetDialogWithContact(ctx, dialogID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated dialog: %w", err)
	}

	// Notify connected clients about the dialog update.
	if u.notifier != nil {
		eventType := EventTypeStatusChanged
		if req.AgentID != nil {
			eventType = EventTypeAssigned
		}
		u.notifier.NotifyDialogUpdated(ctx, &DialogUpdatedEvent{
			DialogID:  dialogID,
			AgentID:   dialog.CurrentAgentID,
			EventType: eventType,
		})
	}

	return dialog, nil
}

// MarkAsRead is a placeholder for marking messages as read in a dialog.
func (u *usecaseImpl) MarkAsRead(ctx context.Context, dialogID, agentID uuid.UUID) error {
	// For now, just verify the dialog exists
	_, err := u.repo.GetDialogByID(ctx, dialogID)
	if err != nil {
		return fmt.Errorf("dialog not found: %w", err)
	}

	u.logger.Info("messages marked as read",
		zap.String("dialog_id", dialogID.String()),
		zap.String("agent_id", agentID.String()),
	)

	return nil
}

// processStatusUpdate handles a delivery/read receipt: updates the message
// status and creates a dialog_event with type "status_updated".
func (u *usecaseImpl) processStatusUpdate(ctx context.Context, channel *Channel, status *parser.StatusUpdate) error {
	// 1. Update message status (forward-only: sent → delivered → read).
	msgStatus := MessageStatus(status.Status)
	if !msgStatus.IsValid() {
		return fmt.Errorf("invalid message status: %s", status.Status)
	}

	if err := u.repo.UpdateMessageStatus(ctx, status.ExternalMessageID, msgStatus); err != nil {
		return fmt.Errorf("failed to update message status (external_id=%s): %w", status.ExternalMessageID, err)
	}

	// 2. Find the message to get dialog_id for the event.
	message, err := u.repo.GetMessageByExternalID(ctx, status.ExternalMessageID)
	if err != nil {
		return fmt.Errorf("failed to find message for status update (external_id=%s): %w", status.ExternalMessageID, err)
	}

	// 3. Create a dialog event.
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

	// Notify connected clients about the status change.
	if u.notifier != nil {
		// Resolve the dialog to get the assigned agent.
		dialog, err := u.repo.GetDialogByID(ctx, message.DialogID)
		var agentID *uuid.UUID
		if err == nil {
			agentID = dialog.CurrentAgentID
		}
		u.notifier.NotifyMessageStatusChanged(ctx, &MessageStatusEvent{
			MessageID: message.ID,
			DialogID:  message.DialogID,
			Status:    msgStatus,
			AgentID:   agentID,
		})
	}

	return nil
}
