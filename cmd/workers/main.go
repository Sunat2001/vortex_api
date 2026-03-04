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
		//startInboundWorker(workerCtx, infra.StreamManager, cfg.Redis.ConsumerGroup, consumerName, logger)
	}()

	// Outbound Messages Worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		//startOutboundWorker(workerCtx, infra.StreamManager, cfg.Redis.ConsumerGroup, consumerName, logger)
	}()

	// Ads Tasks Worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		//startAdsWorker(workerCtx, infra.StreamManager, cfg.Redis.ConsumerGroup, consumerName, logger)
	}()

	// AI Jobs Worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		//startAIWorker(workerCtx, infra.StreamManager, cfg.Redis.ConsumerGroup, consumerName, logger)
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
