package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/voronka/backend/internal/agent"
	agentDelivery "github.com/voronka/backend/internal/agent/delivery"
	"github.com/voronka/backend/internal/app"
	"github.com/voronka/backend/internal/auth"
	authDelivery "github.com/voronka/backend/internal/auth/delivery"
	"github.com/voronka/backend/internal/chat"
	chatDelivery "github.com/voronka/backend/internal/chat/delivery"
	"github.com/voronka/backend/internal/shared/middleware"
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

	logger.Info("starting Vortex API server",
		zap.String("version", "1.0.0"),
		zap.Int("port", cfg.Server.Port),
	)

	// Initialize consumer groups for API-relevant streams only
	streamSetup := app.DefaultAPIStreams()
	if err := app.InitializeConsumerGroups(ctx, infra.StreamManager, cfg.Redis.ConsumerGroup, streamSetup, logger); err != nil {
		logger.Fatal("failed to initialize consumer groups", zap.Error(err))
	}

	// Initialize repositories
	agentRepo := agent.NewRepository(infra.PgPool)
	authRepo := auth.NewRepository(infra.PgPool)

	// Initialize use cases
	agentUsecase := agent.NewUsecase(agentRepo, logger)
	authUsecase := auth.NewUsecase(authRepo, &cfg.JWT, logger)

	// Initialize token validator for middleware
	tokenValidator := auth.NewTokenValidator(authUsecase)
	authMw := middleware.AuthMiddleware(tokenValidator)

	// Set Gin mode
	if cfg.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize Gin router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(logger))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		health := checkSystemHealth(ctx, infra, logger)
		statusCode := http.StatusOK
		if health["status"] != "ok" {
			statusCode = http.StatusServiceUnavailable
		}
		c.JSON(statusCode, health)
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes
		authHandler := authDelivery.NewHTTPHandler(authUsecase, logger)
		authHandler.RegisterRoutes(v1, authMw)

		// Agent routes
		agentHandler := agentDelivery.NewHTTPHandler(agentUsecase, logger)
		agentHandler.RegisterRoutes(v1)

		// Webhook routes (no authentication/RBAC - validation via signatures)
		webhookHandler := chatDelivery.NewWebhookHandler(infra.StreamManager, &cfg.Webhooks, logger)
		webhookHandler.RegisterRoutes(v1)

		// WebSocket hub (must be created before usecase for notifier injection)
		wsHub := chatDelivery.NewHub(logger)
		go wsHub.Run()
		hubNotifier := chatDelivery.NewHubNotifier(wsHub, logger)

		// Chat routes (dialogs, messages)
		chatRepo := chat.NewRepository(infra.PgPool)
		chatUsecase := chat.NewUsecase(chatRepo, hubNotifier, logger)
		chatHandler := chatDelivery.NewHTTPHandler(chatUsecase, logger)
		chatHandler.RegisterRoutes(v1, authMw)

		wsHandler := chatDelivery.NewWSHandler(wsHub, logger)
		wsHandler.RegisterRoutes(v1, authMw)

		// Subscribe to worker events via Redis Pub/Sub → push to local Hub
		go chatDelivery.StartPubSubSubscriber(ctx, infra.RedisClient, wsHub, logger)

		// TODO: Add ads routes
		// TODO: Add catalog routes
	}

	// Start HTTP server
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("HTTP server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}

// Add this function before main() or after imports
func checkSystemHealth(ctx context.Context, infra *app.Infrastructure, logger *zap.Logger) gin.H {
	health := gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"services":  gin.H{},
	}

	services := health["services"].(gin.H)

	// Check PostgreSQL
	if err := infra.PgPool.Ping(ctx); err != nil {
		logger.Error("database health check failed", zap.Error(err))
		services["database"] = "unhealthy"
		health["status"] = "degraded"
	} else {
		services["database"] = "healthy"
	}

	// Check Redis
	if err := infra.RedisClient.Ping(ctx).Err(); err != nil {
		logger.Error("redis health check failed", zap.Error(err))
		services["redis"] = "unhealthy"
		health["status"] = "degraded"
	} else {
		services["redis"] = "healthy"
	}

	return health
}
