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
	ctxInit, cancelInit := context.WithTimeout(context.Background(), 10*time.Second)
	if err := indexManager.EnsureIndices(ctxInit); err != nil {
		slog.Warn("Index initialization deferred or already created", "error", err)
	}
	cancelInit()

	searchRepo := opensearch.NewRepository(osClient)

	// 2. Initialize Redis Cache
	cache := redis.NewCache(cfg)

	// 3. Application Services
	searchService := appSearch.NewService(searchRepo, cache)
	indexingService := indexing.NewService(searchRepo)
	reindexService := reindex.NewService(searchRepo)

	// 4. Kafka Event Consumer Manager
	consumerManager := kafka.NewConsumerManager(cfg, indexingService)
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	if err := consumerManager.Start(consumerCtx); err != nil {
		slog.Error("Failed to start Kafka consumers", "error", err)
	}

	// 5. HTTP & GraphQL Server
	gqlServer := graphql.NewServer(searchService, reindexService)
	healthHandler := health.NewHandler(searchRepo, cache)

	mux := http.NewServeMux()
	mux.HandleFunc("/search/graphql", gqlServer.Handler())
	mux.HandleFunc("/graphql", gqlServer.Handler())
	mux.HandleFunc("/health/live", healthHandler.LivenessHandler())
	mux.HandleFunc("/health/ready", healthHandler.ReadinessHandler())

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.PortGraphQL),
		Handler:      mux,
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

	// 7. Start Main HTTP Server
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
