package delivery

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/voronka/backend/internal/chat"
)

// PubSubChannelWSEvents is the Redis Pub/Sub channel for WebSocket events.
const PubSubChannelWSEvents = "pubsub:ws_events"

// PubSubEvent is the envelope published over Redis Pub/Sub.
type PubSubEvent struct {
	Type    string          `json:"type"`
	AgentID *uuid.UUID      `json:"agent_id,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// PubSubNotifier implements chat.Notifier by publishing events to Redis Pub/Sub.
// Used in the workers process where no WebSocket Hub exists.
type PubSubNotifier struct {
	client *goredis.Client
	logger *zap.Logger
}

// NewPubSubNotifier creates a notifier that publishes to Redis Pub/Sub.
func NewPubSubNotifier(client *goredis.Client, logger *zap.Logger) *PubSubNotifier {
	return &PubSubNotifier{client: client, logger: logger}
}

func (n *PubSubNotifier) NotifyNewMessage(ctx context.Context, event *chat.NewMessageEvent) {
	payload, _ := json.Marshal(event.Message.ToDTO(""))
	n.publish(ctx, "new_message", event.AgentID, payload)
}

func (n *PubSubNotifier) NotifyDialogUpdated(ctx context.Context, event *chat.DialogUpdatedEvent) {
	payload, _ := json.Marshal(map[string]interface{}{
		"dialog_id":  event.DialogID.String(),
		"event_type": string(event.EventType),
	})
	n.publish(ctx, "dialog_updated", event.AgentID, payload)
}

func (n *PubSubNotifier) NotifyMessageStatusChanged(ctx context.Context, event *chat.MessageStatusEvent) {
	payload, _ := json.Marshal(map[string]interface{}{
		"message_id": event.MessageID.String(),
		"dialog_id":  event.DialogID.String(),
		"status":     string(event.Status),
	})
	n.publish(ctx, "message_status", event.AgentID, payload)
}

func (n *PubSubNotifier) publish(ctx context.Context, eventType string, agentID *uuid.UUID, payload json.RawMessage) {
	evt := PubSubEvent{
		Type:    eventType,
		AgentID: agentID,
		Payload: payload,
	}
	data, err := json.Marshal(evt)
	if err != nil {
		n.logger.Error("failed to marshal pubsub event", zap.Error(err))
		return
	}
	if err := n.client.Publish(ctx, PubSubChannelWSEvents, data).Err(); err != nil {
		n.logger.Error("failed to publish ws event",
			zap.String("type", eventType),
			zap.Error(err),
		)
	}
}