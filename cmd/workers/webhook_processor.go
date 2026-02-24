package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/voronka/backend/internal/chat"
	"github.com/voronka/backend/internal/shared/config"
	"github.com/voronka/backend/internal/shared/redis"
)

// PermanentError marks an error that should not be retried.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string { return e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

// inboundRawWorkerPool manages concurrent webhook processing with XCLAIM recovery.
type inboundRawWorkerPool struct {
	sm       *redis.StreamManager
	uc       chat.Usecase
	cfg      *config.RedisConfig
	group    string
	consumer string
	logger   *zap.Logger

	sem chan struct{}
	wg  sync.WaitGroup
}

func startInboundRawWorker(
	ctx context.Context,
	sm *redis.StreamManager,
	uc chat.Usecase,
	cfg *config.RedisConfig,
	group, consumer string,
	logger *zap.Logger,
) {
	pool := &inboundRawWorkerPool{
		sm:       sm,
		uc:       uc,
		cfg:      cfg,
		group:    group,
		consumer: consumer,
		logger:   logger,
		sem:      make(chan struct{}, cfg.WorkerPoolSize),
	}

	logger.Info("inbound raw webhook worker started",
		zap.String("stream", redis.StreamInboundRaw),
		zap.Int("pool_size", cfg.WorkerPoolSize),
	)

	// Start XCLAIM recovery goroutine.
	pool.wg.Add(1)
	go func() {
		defer pool.wg.Done()
		pool.runClaimRecovery(ctx)
	}()

	// Main read loop.
	pool.runMainLoop(ctx)

	// Wait for all in-flight goroutines to finish before returning.
	pool.wg.Wait()
	logger.Info("inbound raw webhook worker stopped")
}

// runMainLoop reads new messages from the stream and dispatches them to the pool.
func (p *inboundRawWorkerPool) runMainLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			streams, err := p.sm.ReadGroupMessage(
				ctx, redis.StreamInboundRaw,
				p.group, p.consumer,
				10, p.cfg.ConsumerBlockTime,
			)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				p.logger.Error("failed to read from inbound raw stream", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					p.dispatch(ctx, msg)
				}
			}
		}
	}
}

// dispatch sends a message to the pool for processing, blocking if all slots are busy.
func (p *inboundRawWorkerPool) dispatch(ctx context.Context, msg goredis.XMessage) {
	select {
	case p.sem <- struct{}{}:
		// Acquired slot.
	case <-ctx.Done():
		return
	}

	p.wg.Add(1)
	go func() {
		defer func() {
			<-p.sem
			p.wg.Done()
		}()

		p.processMessage(ctx, msg)
	}()
}

// processMessage handles a single message: process, ACK, delete.
func (p *inboundRawWorkerPool) processMessage(ctx context.Context, msg goredis.XMessage) {
	processCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := processRawWebhook(processCtx, p.uc, msg.Values, p.logger); err != nil {
		var permErr *PermanentError
		if errors.As(err, &permErr) {
			p.logger.Error("permanent error processing webhook, skipping",
				zap.String("message_id", msg.ID),
				zap.Error(err),
			)
			_ = p.sm.AckMessage(ctx, redis.StreamInboundRaw, p.group, msg.ID)
			_ = p.sm.DeleteMessage(ctx, redis.StreamInboundRaw, msg.ID)
		} else {
			p.logger.Error("transient error processing webhook, will retry via XCLAIM",
				zap.String("message_id", msg.ID),
				zap.Error(err),
			)
			// Don't ACK — message stays in PEL for XCLAIM recovery.
		}
		return
	}

	if err := p.sm.AckMessage(ctx, redis.StreamInboundRaw, p.group, msg.ID); err != nil {
		p.logger.Error("failed to ack message",
			zap.String("message_id", msg.ID),
			zap.Error(err),
		)
	}

	if err := p.sm.DeleteMessage(ctx, redis.StreamInboundRaw, msg.ID); err != nil {
		p.logger.Warn("failed to delete message from stream",
			zap.String("message_id", msg.ID),
			zap.Error(err),
		)
	}
}

// runClaimRecovery periodically scans PEL for stuck messages and reclaims them.
func (p *inboundRawWorkerPool) runClaimRecovery(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.ClaimInterval)
	defer ticker.Stop()

	p.logger.Info("XCLAIM recovery goroutine started",
		zap.Duration("interval", p.cfg.ClaimInterval),
		zap.Duration("min_idle", p.cfg.ClaimMinIdleTime),
		zap.Int64("max_retries", p.cfg.MaxRetryCount),
	)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("XCLAIM recovery goroutine stopped")
			return
		case <-ticker.C:
			p.claimPendingMessages(ctx)
		}
	}
}

// claimPendingMessages executes one cycle of XCLAIM recovery using XAUTOCLAIM.
func (p *inboundRawWorkerPool) claimPendingMessages(ctx context.Context) {
	cursor := "0-0"

	for {
		if ctx.Err() != nil {
			return
		}

		messages, nextCursor, err := p.sm.AutoClaimMessages(
			ctx,
			redis.StreamInboundRaw,
			p.group,
			p.consumer,
			p.cfg.ClaimMinIdleTime,
			cursor,
			p.cfg.ClaimBatchSize,
		)
		if err != nil {
			p.logger.Error("XCLAIM recovery: failed to auto-claim", zap.Error(err))
			return
		}

		if len(messages) == 0 {
			return
		}

		for _, msg := range messages {
			p.handleClaimedMessage(ctx, msg)
		}

		// If cursor returned to "0-0", we've scanned the entire PEL.
		if nextCursor == "0-0" {
			return
		}
		cursor = nextCursor
	}
}

// handleClaimedMessage checks retry count and either reprocesses or dead-letters.
func (p *inboundRawWorkerPool) handleClaimedMessage(ctx context.Context, msg goredis.XMessage) {
	// Check delivery count via XPENDING for this message.
	pending, err := p.sm.GetPendingMessages(
		ctx,
		redis.StreamInboundRaw,
		p.group,
		p.consumer,
		msg.ID,
		msg.ID,
		1,
	)

	retryCount := int64(0)
	if err == nil && len(pending) > 0 {
		retryCount = pending[0].RetryCount
	}

	if retryCount > p.cfg.MaxRetryCount {
		p.logger.Error("message exceeded max retry count, dead-lettering",
			zap.String("message_id", msg.ID),
			zap.Int64("retry_count", retryCount),
			zap.Int64("max_retries", p.cfg.MaxRetryCount),
		)
		p.deadLetter(ctx, msg, fmt.Errorf("exceeded max retry count: %d", retryCount))
		_ = p.sm.AckMessage(ctx, redis.StreamInboundRaw, p.group, msg.ID)
		_ = p.sm.DeleteMessage(ctx, redis.StreamInboundRaw, msg.ID)
		return
	}

	p.logger.Info("retrying claimed message",
		zap.String("message_id", msg.ID),
		zap.Int64("retry_count", retryCount),
	)

	p.dispatch(ctx, msg)
}

// deadLetter moves a failed message to the dead letter stream.
func (p *inboundRawWorkerPool) deadLetter(ctx context.Context, msg goredis.XMessage, reason error) {
	values := map[string]interface{}{
		"original_stream":     redis.StreamInboundRaw,
		"original_message_id": msg.ID,
		"reason":              reason.Error(),
		"dead_lettered_at":    time.Now().Format(time.RFC3339),
	}

	// Copy original message fields.
	for k, v := range msg.Values {
		values["original_"+k] = v
	}

	if _, err := p.sm.AddMessage(ctx, redis.StreamDeadLetters, values); err != nil {
		p.logger.Error("failed to write to dead letter stream",
			zap.String("message_id", msg.ID),
			zap.Error(err),
		)
	}
}

// processRawWebhook unmarshals stream data and calls the usecase.
func processRawWebhook(
	ctx context.Context,
	uc chat.Usecase,
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

	resp, err := uc.ProcessIncomingWebhook(ctx, &chat.ProcessWebhookRequest{
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
