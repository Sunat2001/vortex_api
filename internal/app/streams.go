package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/voronka/backend/internal/shared/redis"
)

// StreamSetup defines which streams to initialize consumer groups for.
// Different services (API, workers) need different subsets of streams.
type StreamSetup struct {
	InboundRaw       bool
	InboundMessages  bool
	OutboundMessages bool
	AdsTasks         bool
	AIJobs           bool
}

// DefaultAPIStreams returns the stream configuration for API server.
// API only needs streams it directly interacts with (receiving webhooks, queueing outbound).
func DefaultAPIStreams() StreamSetup {
	return StreamSetup{
		InboundRaw:       true,  // API receives webhooks and queues to this stream
		InboundMessages:  false, // Processed by workers only
		OutboundMessages: true,  // API may queue outbound messages
		AdsTasks:         false, // Workers only
		AIJobs:           false, // Workers only
	}
}

// DefaultWorkerStreams returns the stream configuration for workers.
// Workers process all streams.
func DefaultWorkerStreams() StreamSetup {
	return StreamSetup{
		InboundRaw:       true,
		InboundMessages:  true,
		OutboundMessages: true,
		AdsTasks:         true,
		AIJobs:           true,
	}
}

// InitializeConsumerGroups creates Redis Stream consumer groups for the specified streams.
// Consumer groups enable at-least-once delivery and allow multiple workers to share the load.
func InitializeConsumerGroups(ctx context.Context, sm *redis.StreamManager, groupName string, setup StreamSetup, logger *zap.Logger) error {
	streams := make([]string, 0, 5)

	if setup.InboundRaw {
		streams = append(streams, redis.StreamInboundRaw)
	}
	if setup.InboundMessages {
		streams = append(streams, redis.StreamInboundMessages)
	}
	if setup.OutboundMessages {
		streams = append(streams, redis.StreamOutboundMessages)
	}
	if setup.AdsTasks {
		streams = append(streams, redis.StreamAdsTasks)
	}
	if setup.AIJobs {
		streams = append(streams, redis.StreamAIJobs)
	}

	for _, stream := range streams {
		if err := sm.CreateConsumerGroup(ctx, stream, groupName); err != nil {
			logger.Error("failed to create consumer group",
				zap.String("stream", stream),
				zap.String("group", groupName),
				zap.Error(err),
			)
			return err
		}
	}

	logger.Info("consumer groups initialized",
		zap.String("group", groupName),
		zap.Int("stream_count", len(streams)),
	)

	return nil
}