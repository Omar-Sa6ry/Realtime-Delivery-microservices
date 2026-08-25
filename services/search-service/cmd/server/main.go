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

	searchRepo := opensearch.NewRepository(osClient, indexManager)

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
	gqlServer, err := graphql.NewServer(searchService, reindexService, cfg.PortGraphQL)
	if err != nil {
		slog.Error("Failed to create GraphQL server", "error", err)
		os.Exit(1)
	}
	healthHandler := health.NewHandler(searchRepo, cache)

	// Health endpoints on separate mux for the main server
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", healthHandler.LivenessHandler())
	mux.HandleFunc("/health/ready", healthHandler.ReadinessHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

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

	// 7. Start Main HTTP Server (health endpoints)
	go func() {
		slog.Info("Search Service listening (health)", "port", cfg.PortGraphQL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Health server error", "error", err)
			os.Exit(1)
		}
	}()

	// 8. Start GraphQL Server (handles /search/graphql)
	go func() {
		slog.Info("Starting GraphQL server", "port", cfg.PortGraphQL)
		if err := gqlServer.Serve(context.Background()); err != nil && err != http.ErrServerClosed {
			slog.Error("GraphQL server error", "error", err)
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
		slog.Error("Health server forced to shutdown", "error", err)
	}

	if err := gqlServer.Shutdown(ctxShutdown); err != nil {
		slog.Error("GraphQL server forced to shutdown", "error", err)
	}

	slog.Info("Search Service stopped.")
}
