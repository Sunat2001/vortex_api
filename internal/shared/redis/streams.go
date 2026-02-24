package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Stream names used across the application
const (
	StreamInboundRaw       = "stream:inbound_raw"       // Raw webhook data from platforms
	StreamInboundMessages  = "stream:inbound_messages"  // Parsed messages (Platform → DB)
	StreamOutboundMessages = "stream:outbound_messages" // Messages to send (DB → Platform)
	StreamAdsTasks         = "stream:ads_tasks"         // Ads sync tasks
	StreamAIJobs           = "stream:ai_jobs"           // AI/MCP processing
	StreamDeadLetters      = "stream:dead_letters"      // Failed messages after max retries
)

// StreamManager handles Redis Stream operations with at-least-once delivery guarantees
type StreamManager struct {
	client *redis.Client
	logger *zap.Logger
	maxLen int64
}

// NewStreamManager creates a new stream manager
func NewStreamManager(client *redis.Client, logger *zap.Logger, maxLen int64) *StreamManager {
	return &StreamManager{
		client: client,
		logger: logger,
		maxLen: maxLen,
	}
}

// AddMessage adds a message to a Redis Stream with MAXLEN to prevent memory overflow
func (sm *StreamManager) AddMessage(ctx context.Context, streamName string, values map[string]interface{}) (string, error) {
	messageID, err := sm.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamName,
		MaxLen: sm.maxLen, // Trim stream to prevent memory issues
		Approx: true,      // Use approximate trimming for better performance
		Values: values,
	}).Result()

	if err != nil {
		sm.logger.Error("failed to add message to stream",
			zap.String("stream", streamName),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to add message to stream %s: %w", streamName, err)
	}

	sm.logger.Debug("message added to stream",
		zap.String("stream", streamName),
		zap.String("message_id", messageID),
	)

	return messageID, nil
}

// CreateConsumerGroup creates a consumer group for a stream (idempotent)
func (sm *StreamManager) CreateConsumerGroup(ctx context.Context, streamName, groupName string) error {
	// Try to create the stream with ID "0" if it doesn't exist
	err := sm.client.XGroupCreateMkStream(ctx, streamName, groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		sm.logger.Error("failed to create consumer group",
			zap.String("stream", streamName),
			zap.String("group", groupName),
			zap.Error(err),
		)
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	sm.logger.Info("consumer group created or already exists",
		zap.String("stream", streamName),
		zap.String("group", groupName),
	)

	return nil
}

// ReadGroupMessage reads messages from a stream using consumer groups (XREADGROUP)
func (sm *StreamManager) ReadGroupMessage(
	ctx context.Context,
	streamName, groupName, consumerName string,
	count int64,
	blockTime time.Duration,
) ([]redis.XStream, error) {
	streams, err := sm.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: consumerName,
		Streams:  []string{streamName, ">"}, // ">" means only new messages
		Count:    count,
		Block:    blockTime, // Block for N milliseconds if no messages
	}).Result()

	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to read from stream: %w", err)
	}

	return streams, nil
}

// AckMessage acknowledges that a message has been processed
func (sm *StreamManager) AckMessage(ctx context.Context, streamName, groupName string, messageIDs ...string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	acked, err := sm.client.XAck(ctx, streamName, groupName, messageIDs...).Result()
	if err != nil {
		sm.logger.Error("failed to acknowledge message",
			zap.String("stream", streamName),
			zap.String("group", groupName),
			zap.Strings("message_ids", messageIDs),
			zap.Error(err),
		)
		return fmt.Errorf("failed to acknowledge message: %w", err)
	}

	sm.logger.Debug("messages acknowledged",
		zap.String("stream", streamName),
		zap.Int64("acked_count", acked),
	)

	return nil
}

// DeleteMessage removes messages from a stream by ID (XDEL).
func (sm *StreamManager) DeleteMessage(ctx context.Context, streamName string, messageIDs ...string) error {
	if len(messageIDs) == 0 {
		return nil
	}

	deleted, err := sm.client.XDel(ctx, streamName, messageIDs...).Result()
	if err != nil {
		sm.logger.Error("failed to delete message from stream",
			zap.String("stream", streamName),
			zap.Strings("message_ids", messageIDs),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete message from stream: %w", err)
	}

	sm.logger.Debug("messages deleted from stream",
		zap.String("stream", streamName),
		zap.Int64("deleted_count", deleted),
	)

	return nil
}

// GetPendingMessages retrieves pending messages that have not been acknowledged
func (sm *StreamManager) GetPendingMessages(
	ctx context.Context,
	streamName, groupName, consumerName string,
	start, end string,
	count int64,
) ([]redis.XPendingExt, error) {
	pending, err := sm.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   streamName,
		Group:    groupName,
		Start:    start,
		End:      end,
		Count:    count,
		Consumer: consumerName,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get pending messages: %w", err)
	}

	return pending, nil
}

// ClaimMessage claims a pending message that another consumer failed to process
func (sm *StreamManager) ClaimMessage(
	ctx context.Context,
	streamName, groupName, consumerName string,
	minIdleTime time.Duration,
	messageIDs ...string,
) ([]redis.XMessage, error) {
	messages, err := sm.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   streamName,
		Group:    groupName,
		Consumer: consumerName,
		MinIdle:  minIdleTime,
		Messages: messageIDs,
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to claim message: %w", err)
	}

	return messages, nil
}

// AutoClaimMessages uses XAUTOCLAIM (Redis 6.2+) to atomically find and claim
// pending messages that have been idle for at least minIdleTime.
// Returns claimed messages, the cursor for next iteration, and any error.
func (sm *StreamManager) AutoClaimMessages(
	ctx context.Context,
	streamName, groupName, consumerName string,
	minIdleTime time.Duration,
	start string,
	count int64,
) (messages []redis.XMessage, nextStart string, err error) {
	msgs, newStart, err := sm.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   streamName,
		Group:    groupName,
		Consumer: consumerName,
		MinIdle:  minIdleTime,
		Start:    start,
		Count:    count,
	}).Result()

	if err != nil {
		sm.logger.Error("failed to auto-claim messages",
			zap.String("stream", streamName),
			zap.String("group", groupName),
			zap.String("consumer", consumerName),
			zap.Duration("min_idle", minIdleTime),
			zap.Error(err),
		)
		return nil, "0-0", fmt.Errorf("failed to auto-claim messages: %w", err)
	}

	if len(msgs) > 0 {
		sm.logger.Info("auto-claimed pending messages",
			zap.String("stream", streamName),
			zap.Int("count", len(msgs)),
			zap.String("next_start", newStart),
		)
	}

	return msgs, newStart, nil
}

// TrimStream trims the stream to the specified max length
func (sm *StreamManager) TrimStream(ctx context.Context, streamName string, maxLen int64) error {
	trimmed, err := sm.client.XTrimMaxLenApprox(ctx, streamName, maxLen, 100).Result()
	if err != nil {
		return fmt.Errorf("failed to trim stream: %w", err)
	}

	sm.logger.Info("stream trimmed",
		zap.String("stream", streamName),
		zap.Int64("trimmed_count", trimmed),
	)

	return nil
}

// GetStreamInfo returns information about a stream
func (sm *StreamManager) GetStreamInfo(ctx context.Context, streamName string) (*redis.XInfoStream, error) {
	info, err := sm.client.XInfoStream(ctx, streamName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	return info, nil
}
