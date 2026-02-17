package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/voronka/backend/internal/chat"
	"github.com/voronka/backend/internal/shared/redis"
)

// PermanentError marks an error that should not be retried.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// startInboundRawWorker consumes raw webhook payloads from Redis and delegates
// all business logic to the chat usecase.
func startInboundRawWorker(
	ctx context.Context,
	sm *redis.StreamManager,
	usecase chat.Usecase,
	group, consumer string,
	logger *zap.Logger,
) {
	logger.Info("inbound raw webhook worker started",
		zap.String("stream", redis.StreamInboundRaw),
	)

	for {
		select {
		case <-ctx.Done():
			logger.Info("inbound raw webhook worker stopped")
			return
		default:
			streams, err := sm.ReadGroupMessage(ctx, redis.StreamInboundRaw, group, consumer, 10, 2*time.Second)
			if err != nil {
				logger.Error("failed to read from inbound raw stream", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := processRawWebhook(ctx, usecase, message.Values, logger); err != nil {
						var permErr *PermanentError
						if errors.As(err, &permErr) {
							logger.Error("permanent error processing webhook, skipping",
								zap.String("message_id", message.ID),
								zap.Error(err),
							)
							// ACK to avoid infinite retry of broken messages.
							_ = sm.AckMessage(ctx, redis.StreamInboundRaw, group, message.ID)
						} else {
							logger.Error("transient error processing webhook, will retry",
								zap.String("message_id", message.ID),
								zap.Error(err),
							)
							// Don't ACK — message stays in pending list for retry.
						}
						continue
					}

					if err := sm.AckMessage(ctx, redis.StreamInboundRaw, group, message.ID); err != nil {
						logger.Error("failed to ack message",
							zap.String("message_id", message.ID),
							zap.Error(err),
						)
					}

					// Remove processed message from stream to free memory.
					if err := sm.DeleteMessage(ctx, redis.StreamInboundRaw, message.ID); err != nil {
						logger.Warn("failed to delete message from stream",
							zap.String("message_id", message.ID),
							zap.Error(err),
						)
					}
				}
			}
		}
	}
}

// processRawWebhook unmarshals stream data and calls the usecase.
func processRawWebhook(
	ctx context.Context,
	usecase chat.Usecase,
	data map[string]interface{},
	logger *zap.Logger,
) error {
	source, ok := data["source"].(string)
	if !ok {
		return &PermanentError{Err: fmt.Errorf("missing source field")}
	}

	payloadStr, ok := data["payload"].(string)
	if !ok {
		return &PermanentError{Err: fmt.Errorf("missing payload field")}
	}

	// Validate JSON before passing to usecase.
	if !json.Valid([]byte(payloadStr)) {
		return &PermanentError{Err: fmt.Errorf("invalid JSON payload")}
	}

	receivedAtStr, _ := data["received_at"].(string)
	receivedAt, _ := time.Parse(time.RFC3339, receivedAtStr)
	if receivedAt.IsZero() {
		receivedAt = time.Now()
	}

	platform := mapSourceToPlatform(source)

	logger.Info("processing webhook",
		zap.String("platform", string(platform)),
		zap.String("received_at", receivedAt.Format(time.RFC3339)),
		zap.Int("payload_size", len(payloadStr)),
	)

	resp, err := usecase.ProcessIncomingWebhook(ctx, &chat.ProcessWebhookRequest{
		Platform:   platform,
		RawPayload: json.RawMessage(payloadStr),
		ReceivedAt: receivedAt,
	})
	if err != nil {
		if errors.Is(err, chat.ErrNoChannel) {
			return &PermanentError{Err: err}
		}
		return err
	}

	logger.Info("webhook processed",
		zap.String("platform", string(platform)),
		zap.Int("messages_created", resp.MessagesCreated),
	)

	return nil
}

func mapSourceToPlatform(source string) chat.Platform {
	switch strings.ToLower(source) {
	case "facebook":
		return chat.PlatformFacebook
	case "instagram":
		return chat.PlatformInstagram
	case "whatsapp":
		return chat.PlatformWhatsApp
	default:
		return chat.Platform(source)
	}
}