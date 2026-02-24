package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/voronka/backend/internal/app"
	"github.com/voronka/backend/internal/chat"
	"github.com/voronka/backend/internal/shared/redis"
)

func main() {
	// Bootstrap infrastructure (config, logger, database, redis)
	ctx := context.Background()
	infra, err := app.BootstrapInfrastructure(ctx)
	if err != nil {
		panic(fmt.Sprintf("failed to bootstrap infrastructure: %v", err))
	}
	defer infra.Close()

	logger := infra.Logger
	cfg := infra.Config

	logger.Info("starting Vortex Workers",
		zap.String("version", "1.0.0"),
		zap.String("consumer_group", cfg.Redis.ConsumerGroup),
	)

	// Initialize consumer groups for all worker streams
	streamSetup := app.DefaultWorkerStreams()
	if err := app.InitializeConsumerGroups(ctx, infra.StreamManager, cfg.Redis.ConsumerGroup, streamSetup, logger); err != nil {
		logger.Fatal("failed to initialize consumer groups", zap.Error(err))
	}

	// Initialize repositories and usecases
	chatRepo := chat.NewRepository(infra.PgPool)
	chatUsecase := chat.NewUsecase(chatRepo, logger)

	// Create worker context
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	// Wait group for graceful shutdown
	var wg sync.WaitGroup

	// Start worker pools
	consumerName := fmt.Sprintf("worker-%s", uuid.New().String()[:8])

	// Inbound Raw Webhook Worker (processes raw webhook data from platforms)
	wg.Add(1)
	go func() {
		defer wg.Done()
		startInboundRawWorker(workerCtx, infra.StreamManager, chatUsecase, &cfg.Redis, cfg.Redis.ConsumerGroup, consumerName, logger)
	}()

	// Inbound Messages Worker (processes parsed messages)
	wg.Add(1)
	go func() {
		defer wg.Done()
		startInboundWorker(workerCtx, infra.StreamManager, cfg.Redis.ConsumerGroup, consumerName, logger)
	}()

	// Outbound Messages Worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		startOutboundWorker(workerCtx, infra.StreamManager, cfg.Redis.ConsumerGroup, consumerName, logger)
	}()

	// Ads Tasks Worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		startAdsWorker(workerCtx, infra.StreamManager, cfg.Redis.ConsumerGroup, consumerName, logger)
	}()

	// AI Jobs Worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		startAIWorker(workerCtx, infra.StreamManager, cfg.Redis.ConsumerGroup, consumerName, logger)
	}()

	logger.Info("all workers started", zap.String("consumer_name", consumerName))

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down workers...")

	// Cancel worker context
	workerCancel()

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("all workers stopped gracefully")
	case <-time.After(30 * time.Second):
		logger.Warn("workers did not stop in time, forcing shutdown")
	}

	logger.Info("workers exited")
}

// startInboundWorker processes incoming messages from messaging platforms
func startInboundWorker(ctx context.Context, sm *redis.StreamManager, group, consumer string, logger *zap.Logger) {
	logger.Info("inbound worker started", zap.String("stream", redis.StreamInboundMessages))

	for {
		select {
		case <-ctx.Done():
			logger.Info("inbound worker stopped")
			return
		default:
			// Read messages from stream
			streams, err := sm.ReadGroupMessage(
				ctx,
				redis.StreamInboundMessages,
				group,
				consumer,
				10, // Read up to 10 messages at once
				2*time.Second,
			)

			if err != nil {
				logger.Error("failed to read from inbound stream", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			// Process messages
			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := processInboundMessage(ctx, message.Values, logger); err != nil {
						logger.Error("failed to process inbound message",
							zap.String("message_id", message.ID),
							zap.Error(err),
						)
						continue
					}

					// Acknowledge message
					if err := sm.AckMessage(ctx, redis.StreamInboundMessages, group, message.ID); err != nil {
						logger.Error("failed to ack message", zap.String("message_id", message.ID), zap.Error(err))
					}
				}
			}
		}
	}
}

// startOutboundWorker processes outgoing messages to messaging platforms
func startOutboundWorker(ctx context.Context, sm *redis.StreamManager, group, consumer string, logger *zap.Logger) {
	logger.Info("outbound worker started", zap.String("stream", redis.StreamOutboundMessages))

	for {
		select {
		case <-ctx.Done():
			logger.Info("outbound worker stopped")
			return
		default:
			streams, err := sm.ReadGroupMessage(
				ctx,
				redis.StreamOutboundMessages,
				group,
				consumer,
				10,
				2*time.Second,
			)

			if err != nil {
				logger.Error("failed to read from outbound stream", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := processOutboundMessage(ctx, message.Values, logger); err != nil {
						logger.Error("failed to process outbound message",
							zap.String("message_id", message.ID),
							zap.Error(err),
						)
						continue
					}

					if err := sm.AckMessage(ctx, redis.StreamOutboundMessages, group, message.ID); err != nil {
						logger.Error("failed to ack message", zap.String("message_id", message.ID), zap.Error(err))
					}
				}
			}
		}
	}
}

// startAdsWorker processes ads-related tasks
func startAdsWorker(ctx context.Context, sm *redis.StreamManager, group, consumer string, logger *zap.Logger) {
	logger.Info("ads worker started", zap.String("stream", redis.StreamAdsTasks))

	for {
		select {
		case <-ctx.Done():
			logger.Info("ads worker stopped")
			return
		default:
			streams, err := sm.ReadGroupMessage(
				ctx,
				redis.StreamAdsTasks,
				group,
				consumer,
				5,
				2*time.Second,
			)

			if err != nil {
				logger.Error("failed to read from ads stream", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := processAdsTask(ctx, message.Values, logger); err != nil {
						logger.Error("failed to process ads task",
							zap.String("message_id", message.ID),
							zap.Error(err),
						)
						continue
					}

					if err := sm.AckMessage(ctx, redis.StreamAdsTasks, group, message.ID); err != nil {
						logger.Error("failed to ack message", zap.String("message_id", message.ID), zap.Error(err))
					}
				}
			}
		}
	}
}

// startAIWorker processes AI-related jobs (MCP integration)
func startAIWorker(ctx context.Context, sm *redis.StreamManager, group, consumer string, logger *zap.Logger) {
	logger.Info("AI worker started", zap.String("stream", redis.StreamAIJobs))

	for {
		select {
		case <-ctx.Done():
			logger.Info("AI worker stopped")
			return
		default:
			streams, err := sm.ReadGroupMessage(
				ctx,
				redis.StreamAIJobs,
				group,
				consumer,
				5,
				2*time.Second,
			)

			if err != nil {
				logger.Error("failed to read from AI stream", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := processAIJob(ctx, message.Values, logger); err != nil {
						logger.Error("failed to process AI job",
							zap.String("message_id", message.ID),
							zap.Error(err),
						)
						continue
					}

					if err := sm.AckMessage(ctx, redis.StreamAIJobs, group, message.ID); err != nil {
						logger.Error("failed to ack message", zap.String("message_id", message.ID), zap.Error(err))
					}
				}
			}
		}
	}
}

// Message processing functions (to be implemented)

func processInboundMessage(ctx context.Context, data map[string]interface{}, logger *zap.Logger) error {
	// TODO: Implement inbound message processing
	// 1. Parse message from platform
	// 2. Create/update contact
	// 3. Create/update dialog
	// 4. Save message to database
	// 5. Trigger AI analysis if needed
	logger.Debug("processing inbound message", zap.Any("data", data))
	return nil
}

func processOutboundMessage(ctx context.Context, data map[string]interface{}, logger *zap.Logger) error {
	// TODO: Implement outbound message processing
	// 1. Get message details from database
	// 2. Apply rate limiting
	// 3. Send to platform API (Telegram, WhatsApp, etc.)
	// 4. Update message status
	// 5. Handle delivery confirmation via webhook
	logger.Debug("processing outbound message", zap.Any("data", data))
	return nil
}

func processAdsTask(ctx context.Context, data map[string]interface{}, logger *zap.Logger) error {
	// TODO: Implement ads task processing
	// 1. Sync campaign data from Meta/Google
	// 2. Fetch performance metrics
	// 3. Calculate liquidity scores
	// 4. Update database
	logger.Debug("processing ads task", zap.Any("data", data))
	return nil
}

func processAIJob(ctx context.Context, data map[string]interface{}, logger *zap.Logger) error {
	// TODO: Implement AI job processing
	// 1. Extract intent from message
	// 2. Query catalog via MCP tools
	// 3. Generate response draft
	// 4. Send to WebSocket for agent review
	logger.Debug("processing AI job", zap.Any("data", data))
	return nil
}
