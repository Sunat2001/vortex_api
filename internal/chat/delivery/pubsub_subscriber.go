package delivery

import (
	"context"
	"encoding/json"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// StartPubSubSubscriber listens for WebSocket events published by workers
// and relays them to the local Hub. Run as a goroutine in the API process.
func StartPubSubSubscriber(ctx context.Context, client *goredis.Client, hub *Hub, logger *zap.Logger) {
	sub := client.Subscribe(ctx, PubSubChannelWSEvents)
	defer sub.Close()

	ch := sub.Channel()
	logger.Info("pubsub ws subscriber started", zap.String("channel", PubSubChannelWSEvents))

	for {
		select {
		case <-ctx.Done():
			logger.Info("pubsub ws subscriber stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				logger.Warn("pubsub channel closed")
				return
			}
			handlePubSubMessage(hub, msg, logger)
		}
	}
}

func handlePubSubMessage(hub *Hub, msg *goredis.Message, logger *zap.Logger) {
	var evt PubSubEvent
	if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
		logger.Error("failed to unmarshal pubsub event", zap.Error(err))
		return
	}

	wsEvent := &WSEvent{
		Type: evt.Type,
		Data: json.RawMessage(evt.Payload),
	}

	if evt.AgentID != nil {
		hub.SendToUser(*evt.AgentID, wsEvent)
	} else {
		hub.Broadcast(wsEvent)
	}
}