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

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/config"
	gql "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/graphql"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", gql.HealthLiveHandler)
	mux.HandleFunc("/health/ready", gql.HealthReadyHandler)
	mux.Handle("/metrics", promhttp.Handler())

	// GraphQL endpoints:
	// - /graphql for direct local testing
	// - /analytics/graphql for API Gateway (ANALYTICS_SERVICE_URL=http://analytics-srv:4009/analytics/graphql)
	mux.HandleFunc("/graphql", gql.GraphQLHandler)
	mux.HandleFunc("/analytics/graphql", gql.GraphQLHandler)

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
