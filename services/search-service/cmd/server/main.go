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
	sharedRabbitMQ "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/rabbitmq"
	sharedtracing "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/tracing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/indexing"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/reindex"
	appSearch "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/application/search"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/kafka"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/opensearch"
	searchRabbitMQ "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/rabbitmq"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/infrastructure/redis"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/interfaces/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/interfaces/health"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/observability"
)

func main() {
	os.Exit(runMain(context.Background()))
}

// runMain wires OS-interrupt signals onto parent and returns the process exit code.
func runMain(parent context.Context) int {
	logging.InitLogger()
	slog.Info("Starting Search Service...")

	if err := sharedenv.Load(); err != nil {
		slog.Error("Failed to load environment", "error", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("Search service failed", "error", err)
		return 1
	}
	return 0
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// OTLP tracer (no endpoint -> the service runs untraced).
	tracer, err := observability.NewTracer(observability.TracerConfig{
		ServiceName:  "search-service",
		OTLPEndpoint: cfg.OTELEndpoint,
		SamplingRate: cfg.OTELSamplingRate,
	})
	if err != nil {
		slog.Warn("Tracer init failed (non-fatal)", "error", err)
	}

	// 1. Initialize OpenSearch
	osClient, err := opensearch.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("connect to OpenSearch: %w", err)
	}

	indexManager := opensearch.NewIndexManager(osClient)

	// Wait for OpenSearch indices to be ready before starting Kafka consumers.
	maxRetries := cfg.IndexInitMaxRetries
	for i := 0; i < maxRetries; i++ {
		ctxInit, cancelInit := context.WithTimeout(ctx, 5*time.Second)
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
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cfg.IndexInitRetryDelay):
			}
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
	// Start never returns a non-nil error (consumers run in background goroutines).
	_ = consumerManager.Start(ctx)

	// 5b. RabbitMQ Event Consumer (dual-consumption / fast events)
	var rmqConn *sharedRabbitMQ.Connection
	if cfg.RabbitMQURL != "" {
		var err error
		rmqConn, err = sharedRabbitMQ.Connect(sharedRabbitMQ.Config{
			URL:         cfg.RabbitMQURL,
			ServiceName: "search-service",
			MaxAttempts: cfg.RabbitMQMaxAttempts,
			BaseDelay:   cfg.RabbitMQBaseDelay,
		})
		if err != nil {
			slog.Warn("Failed to connect to RabbitMQ (search-service will continue with Kafka only)", "error", err)
		} else {
			rmqConsumer := searchRabbitMQ.NewSearchRabbitMQConsumer(rmqConn, indexingService)
			go func() {
				if err := rmqConsumer.Start(ctx); err != nil {
					slog.Error("RabbitMQ search consumer stopped", "error", err)
				}
			}()
			slog.Info("RabbitMQ search index consumer started successfully")
		}
	}

	// 6. HTTP & GraphQL Server
	gqlServer, err := graphql.NewServer(searchService, reindexService, cfg.PortGraphQL)
	if err != nil {
		return fmt.Errorf("create GraphQL server: %w", err)
	}
	healthHandler := health.NewHandler(searchRepo, cache)

	var handler http.Handler = buildHandler(healthHandler, gqlServer)
	handler = metrics.HTTPMetricsMiddleware(handler)
	handler = sharedtracing.HTTPTracingMiddleware(handler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.PortGraphQL),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// 6b. Prometheus Metrics Server
	go func() {
		slog.Info("Starting metrics server", "port", cfg.PortMetrics)
		if err := metrics.StartMetricsServer(cfg.PortMetrics); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server failed", "error", err)
		}
	}()

	// 7. Start Main HTTP Server (health + GraphQL endpoints)
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("Search Service listening", "port", cfg.PortGraphQL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// 8. Wait for shutdown signal or server failure
	select {
	case <-ctx.Done():
		slog.Info("Shutting down Search Service gracefully...")
	case err := <-serverErr:
		_ = consumerManager.Close()
		if rmqConn != nil {
			_ = rmqConn.Close()
		}
		return fmt.Errorf("http server: %w", err)
	}

	_ = consumerManager.Close()
	if rmqConn != nil {
		_ = rmqConn.Close()
	}

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	if tracer != nil {
		ctxTracer, cancelTracer := context.WithTimeout(context.Background(), 5*time.Second)
		if err := tracer.Shutdown(ctxTracer); err != nil {
			slog.Warn("Tracer shutdown failed", "error", err)
		}
		cancelTracer()
	}

	slog.Info("Search Service stopped.")
	return nil
}

// buildHandler assembles the production HTTP mux (health + GraphQL routes).
func buildHandler(healthHandler *health.Handler, gqlServer *graphql.Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", healthHandler.LivenessHandler())
	mux.HandleFunc("/health/ready", healthHandler.ReadinessHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/search/graphql", gqlServer.Handler())
	mux.HandleFunc("/graphql", gqlServer.Handler())
	return mux
}
