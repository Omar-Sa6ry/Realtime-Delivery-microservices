package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/realtime-delivery/payment-service/internal/config"
	"github.com/realtime-delivery/payment-service/internal/graphql"
	"github.com/realtime-delivery/payment-service/internal/observability"
	"github.com/realtime-delivery/payment-service/internal/workers"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Set Gin mode
	if cfg.NodeEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize observability
	metrics := observability.NewMetrics()
	tracer, err := observability.NewTracer(observability.TracerConfig{
		ServiceName:    "payment-service",
		OTLPEndpoint:   cfg.OTELEndpoint,
		SamplingRate:   0.1,
		EnableInsecure: true,
	})
	if err != nil {
		logger.Warn("Failed to initialize tracer", zap.Error(err))
		// Continue without tracer
	}
	defer func() {
		if tracer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tracer.Shutdown(ctx)
		}
	}()

	// Initialize GraphQL
	graphqlHandler := graphql.GraphQLHandler()

	// Initialize Worker Pool
	workerPool := workers.NewWorkerPool(5 * time.Second)

	// Create Gin router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Health check endpoints
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	router.GET("/health/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// GraphQL endpoint
	router.POST("/payment/graphql", graphqlHandler)
	router.GET("/payment/graphql", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Payment Service GraphQL endpoint. Use POST with query.",
		})
	})

	// Webhook endpoint
	router.POST("/webhook/payment", func(c *gin.Context) {
		metrics.RecordWebhookEvent("payment", "received")
		c.JSON(http.StatusOK, gin.H{"status": "received"})
	})

	// Start HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.PortGraphQL,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start Worker Pool
	workerPool.Start()

	// Run server in goroutine
	go func() {
		logger.Info("Starting Payment Service",
			zap.String("port", cfg.PortGraphQL),
			zap.String("env", cfg.NodeEnv),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Stop worker pool gracefully
	workerPool.Stop()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

// initializeDependencies initializes all service dependencies
func initializeDependencies(cfg *config.Config, logger *zap.Logger, metrics *observability.Metrics, tracer *observability.Tracer) (*workers.WorkerPool, error) {
	// TODO: Initialize database connection
	// TODO: Initialize Redis client
	// TODO: Initialize Kafka producer/consumer
	// TODO: Initialize NATS connection
	// TODO: Initialize payment provider (Stripe)
	// TODO: Initialize repositories
	// TODO: Initialize worker pool with actual dependencies

	// For now, return a basic worker pool
	workerPool := workers.NewWorkerPool(5 * time.Second)
	return workerPool, nil
}