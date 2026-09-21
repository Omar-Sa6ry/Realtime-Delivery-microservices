package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/adapters/clickhouse"
	redisadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/adapters/redis"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/analytics"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/reconciliation"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/config"
	gql "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/workers"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// ClickHouse is required: facts, queries, and idempotency live there.
	chClient, err := clickhouse.NewClient(clickhouse.Config{
		Host:     cfg.ClickHouseHost,
		Port:     cfg.ClickHousePort,
		Database: cfg.ClickHouseDB,
		Username: cfg.ClickHouseUser,
		Password: cfg.ClickHousePassword,
	})
	if err != nil {
		slog.Error("failed to create clickhouse client", "error", err)
		os.Exit(1)
	}
	defer chClient.Close()
	if err := chClient.Ping(context.Background()); err != nil {
		slog.Error("clickhouse unreachable", "error", err)
		os.Exit(1)
	}
	if err := clickhouse.Migrate(context.Background(), chClient); err != nil {
		slog.Error("clickhouse migrations failed", "error", err)
		os.Exit(1)
	}

	// Redis is optional: query caching degrades to direct ClickHouse reads.
	cache := redisadapter.NewCache(cfg.RedisHost, cfg.RedisPort)
	defer cache.Close()

	queryRepo := clickhouse.NewQueryRepository(chClient)
	cacheTTL := time.Duration(cfg.CacheTTLSeconds) * time.Second

	resolver := gql.NewResolver(
		analytics.NewPlatformOverviewService(queryRepo, cache, cacheTTL),
		analytics.NewDeliveryAnalyticsService(queryRepo, cache, cacheTTL),
		analytics.NewDriverAnalyticsService(queryRepo, cache, cacheTTL),
		analytics.NewPaymentAnalyticsService(queryRepo, cache, cacheTTL),
		queryRepo,
	)

	// Background workers: periodic reconciliation cron (observe-only).
	pool := workers.NewPool()
	reconciliationSvc := reconciliation.NewService(queryRepo, reconciliation.Config{WindowHours: 1})
	pool.Register("reconciliation", time.Duration(cfg.ReconcileIntervalSec)*time.Second,
		workers.NewReconciliationWorker(reconciliationSvc).Run)
	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	pool.Start(workerCtx)
	defer pool.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", gql.HealthLiveHandler)
	mux.HandleFunc("/health/ready", gql.HealthReadyHandler(chClient.Ping))
	mux.Handle("/metrics", promhttp.Handler())

	// GraphQL endpoints:
	// - /graphql for direct local testing
	// - /analytics/graphql for API Gateway (ANALYTICS_SERVICE_URL=http://analytics-srv:4009/analytics/graphql)
	gqlHandler := gql.DataLoaderMiddleware(gql.GraphQLHandler(resolver))
	mux.Handle("/graphql", gqlHandler)
	mux.Handle("/analytics/graphql", gqlHandler)

	handler := gql.LanguageMiddleware(mux)

	mainSrv := &http.Server{
		Addr:         ":" + cfg.PortGraphQL,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Metrics server (port 9107) — same promhttp handler on a separate port
	// so Prometheus can scrape without going through the main mux.
	metricsSrv := &http.Server{
		Addr:         ":" + cfg.PortMetrics,
		Handler:      promhttp.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("metrics server starting", "port", cfg.PortMetrics)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	go func() {
		slog.Info("analytics service starting", "port", cfg.PortGraphQL, "env", cfg.NodeEnv)
		if err := mainSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("main server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down analytics service...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = mainSrv.Shutdown(ctx)
	_ = metricsSrv.Shutdown(ctx)
	slog.Info("analytics service stopped")
}
