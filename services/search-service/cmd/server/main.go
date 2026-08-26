package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sharedenv "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/env"
	pkgKafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/metrics"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/reindex"
	appSearch "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/opensearch"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/redis"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/interfaces/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/interfaces/health"
)

func main() {
	logging.InitLogger()
	slog.Info("Starting Search Service...")

	if err := sharedenv.Load(); err != nil {
		slog.Error("Failed to load environment", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// 1. Initialize OpenSearch
	osClient, err := opensearch.NewClient(cfg)
	if err != nil {
		slog.Error("Failed to connect to OpenSearch", "error", err)
		os.Exit(1)
	}

	indexManager := opensearch.NewIndexManager(osClient)

	// Wait for OpenSearch indices to be ready before starting Kafka consumers.
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		ctxInit, cancelInit := context.WithTimeout(context.Background(), 5*time.Second)
		err := indexManager.EnsureIndices(ctxInit)
		cancelInit()
		if err == nil {
			slog.Info("OpenSearch indices ready")
			break
		}
		if i == maxRetries-1 {
			slog.Warn("Index initialization failed after max retries, continuing anyway", "error", err)
		} else {
			slog.Warn("Waiting for OpenSearch indices", "attempt", i+1, "maxRetries", maxRetries, "error", err)
			time.Sleep(3 * time.Second)
		}
	}

	searchRepo := opensearch.NewRepository(osClient, indexManager)

	// 2. Initialize Redis Cache
	cache := redis.NewCache(cfg)

	// 3. Application Services
	searchService := appSearch.NewService(searchRepo, cache)
	indexingService := indexing.NewService(searchRepo)
	reindexService := reindex.NewService(searchRepo)

	// 4. Ensure Kafka topics exist before starting consumers.

	searchTopics := []string{
		"delivery.created", "delivery.driver.assigned", "delivery.driver.accepted",
		"delivery.picked_up", "delivery.in_transit", "delivery.completed",
		"delivery.cancelled", "delivery.deleted",
		"driver.created", "driver.updated", "driver.deleted",
		"media.upload.created", "media.upload.completed", "media.ready", "media.deleted",
		"user.created", "user.updated", "user.deleted",
	}
	if err := pkgKafka.EnsureTopics(cfg.KafkaBrokers, searchTopics, 1, 1); err != nil {
		slog.Warn("Failed to ensure Kafka topics (will retry on consumer start)", "error", err)
	} else {
		slog.Info("Kafka topics verified", "count", len(searchTopics))
	}

	// 5. Kafka Event Consumer Manager
	consumerManager := kafka.NewConsumerManager(cfg, indexingService)
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	if err := consumerManager.Start(consumerCtx); err != nil {
		slog.Error("Failed to start Kafka consumers", "error", err)
	}

	// 6. HTTP & GraphQL Server - Create GraphQL server first to get its handler
	gqlServer, err := graphql.NewServer(searchService, reindexService, cfg.PortGraphQL)
	if err != nil {
		slog.Error("Failed to create GraphQL server", "error", err)
		os.Exit(1)
	}
	healthHandler := health.NewHandler(searchRepo, cache)

	// Single mux for both health and GraphQL endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", healthHandler.LivenessHandler())
	mux.HandleFunc("/health/ready", healthHandler.ReadinessHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// GraphQL endpoints - delegate to GraphQL server's handler
	mux.HandleFunc("/search/graphql", gqlServer.Handler())
	mux.HandleFunc("/graphql", gqlServer.Handler())

	// Wrap with metrics middleware
	var handler http.Handler = mux
	handler = metrics.HTTPMetricsMiddleware(handler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.PortGraphQL),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// 6. Prometheus Metrics Server
	go func() {
		slog.Info("Starting metrics server", "port", cfg.PortMetrics)
		if err := metrics.StartMetricsServer(cfg.PortMetrics); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	// 7. Start Main HTTP Server (health + GraphQL endpoints)
	go func() {
		slog.Info("Search Service listening", "port", cfg.PortGraphQL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// 8. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down Search Service gracefully...")
	consumerCancel()
	_ = consumerManager.Close()

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Search Service stopped.")
}
