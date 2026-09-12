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
	"go.uber.org/zap"

	"github.com/realtime-delivery/payment-service/internal/config"
	"github.com/realtime-delivery/payment-service/internal/graphql"
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

	// Create Gin router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Health check endpoint
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})
	router.GET("/health/ready", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// GraphQL endpoint
	router.POST("/payment/graphql", graphql.GraphQLHandler())
	router.GET("/payment/graphql", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Payment Service GraphQL endpoint. Use POST with query.",
		})
	})

	// Webhook endpoint (placeholder)
	router.POST("/webhook/payment", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "received"})
	})

	// Metrics endpoint (placeholder)
	router.GET("/metrics", func(c *gin.Context) {
		c.String(http.StatusOK, "# Payment service metrics\n")
	})

	// Start HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.PortGraphQL,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}