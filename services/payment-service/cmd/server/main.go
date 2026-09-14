package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	// packages/go shared utilities
	pkglogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	pkgmetrics "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/metrics"
	pkgsnowflake "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/snowflake"

	// internal packages
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/kafka"
	natsadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/nats"
	pgadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/postgres"
	stripeadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/providers/stripe"
	webhookhandler "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/webhook"
	grpcserver "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/adapters/grpc"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/config"
	gql "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/workers"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/application/services"
)

func main() {
	// Shared Logger (packages/go/logging)
	logger := pkglogging.InitLogger()
	_ = logger // slog.Default() is set inside InitLogger

	// Configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if cfg.NodeEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Snowflake ID Generator (packages/go/snowflake)
	sf, err := pkgsnowflake.NewSnowflake(pkgsnowflake.Config{
		WorkerID: cfg.SnowflakeWorkerID,
	})
	if err != nil {
		slog.Error("failed to initialize snowflake", "error", err)
		os.Exit(1)
	}
	_ = sf // passed to application services

	// PostgreSQL 
	db, err := pgadapter.Connect(cfg.PostgresDSN)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pgadapter.Close(db)

	// Repositories
	paymentRepo := pgadapter.NewPaymentRepository(db)
	outboxRepo := pgadapter.NewOutboxRepository(db)
	refundRepo := pgadapter.NewRefundRepository(db)
	attemptRepo := pgadapter.NewAttemptRepository(db)

	// Stripe Provider
	stripeProvider := stripeadapter.NewStripeProvider(cfg.StripeSecretKey)

	// Kafka Event Publisher 
	brokers := splitBrokers(cfg.KafkaBrokers)
	if err := kafka.EnsureTopics(brokers); err != nil {
		slog.Warn("kafka topics setup failed (non-fatal)", "error", err)
	}
	kafkaPublisher := kafka.NewEventPublisher(brokers, "payment-events")
	defer kafkaPublisher.Close()

	// NATS Realtime Publisher
	natsPublisher, err := natsadapter.NewRealtimePublisher(cfg.NATSUrl)
	if err != nil {
		slog.Warn("NATS connection failed (non-fatal — realtime disabled)", "error", err)
	}
	if natsPublisher != nil {
		defer natsPublisher.Close()
	}

	// Observability
	paymentMetrics := observability.NewMetrics()
	tracer, err := observability.NewTracer(observability.TracerConfig{
		ServiceName:    "payment-service",
		OTLPEndpoint:   cfg.OTELEndpoint,
		SamplingRate:   0.1,
		EnableInsecure: true,
	})
	if err != nil {
		slog.Warn("tracer init failed (non-fatal)", "error", err)
	}
	if tracer != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tracer.Shutdown(ctx)
		}()
	}
	_ = paymentMetrics

	// Application Service
	paymentSvc := services.NewPaymentService(
		paymentRepo,
		refundRepo,
		attemptRepo,
		outboxRepo,
		stripeProvider,
		natsPublisher,
		sf,
	)

	//  11. Background Workers 
	pool := workers.NewWorkerPool()

	outboxWorker := workers.NewOutboxPublisher(outboxRepo, kafkaPublisher, cfg.OutboxBatchSize)
	pool.Register("outbox-publisher", 2*time.Second, outboxWorker.Run)

	reconcileWorker := workers.NewReconciliationWorker(attemptRepo, paymentRepo, stripeProvider)
	pool.Register("reconciliation", cfg.ReconcileIntervalSec, reconcileWorker.Run)

	stuckWorker := workers.NewStuckRecoveryWorker(attemptRepo, int(cfg.StuckThresholdSec.Seconds()))
	pool.Register("stuck-recovery", cfg.StuckThresholdSec, stuckWorker.Run)

	cleanupWorker := workers.NewCleanupWorker(outboxRepo)
	pool.Register("cleanup", 1*time.Hour, cleanupWorker.Run)

	pool.Start()
	defer pool.Stop()

	//  Metrics HTTP Server (port 9106) 
	// Uses packages/go/metrics.StartMetricsServer which registers Prometheus handler.
	go func() {
		slog.Info("metrics server starting", "port", cfg.PortMetrics)
		if err := pkgmetrics.StartMetricsServer(cfg.PortMetrics); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server failed", "error", err)
		}
	}()

	//  gRPC Server (port 50056) 
	grpcSrv := grpcserver.NewGRPCServer(
		cfg.PortGRPC,
		paymentSvc,
		pkgmetrics.UnaryServerMetricsInterceptor(), // from packages/go/metrics
	)
	go func() {
		slog.Info("gRPC server starting", "port", cfg.PortGRPC)
		lis, err := net.Listen("tcp", ":"+cfg.PortGRPC)
		if err != nil {
			slog.Error("gRPC listen failed", "error", err)
			return
		}
		if err := grpcSrv.Start(lis); err != nil && err != grpc.ErrServerStopped {
			slog.Error("gRPC server failed", "error", err)
		}
	}()
	defer grpcSrv.Stop()

	// Stripe Webhook Server (port 4012) 
	webhookHandler := webhookhandler.NewHandler(
		cfg.StripeWebhookSecret,
		paymentRepo,
		outboxRepo,
	)
	webhookRouter := gin.New()
	webhookRouter.Use(gin.Recovery())
	webhookRouter.POST("/webhook/payment", webhookHandler.Handle)
	webhookSrv := &http.Server{
		Addr:         ":" + cfg.PortWebhook,
		Handler:      webhookRouter,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	go func() {
		slog.Info("webhook server starting", "port", cfg.PortWebhook)
		if err := webhookSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("webhook server failed", "error", err)
		}
	}()

	//  GraphQL / Main HTTP Server (port 4002) ─
	router := gin.New()
	router.Use(gin.Recovery())

	// Attach shared metrics handler (packages/go/metrics promhttp)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Health endpoints
	router.GET("/health/live", gql.HealthHandler())
	router.GET("/health/ready", gql.ReadyHandler(db))

	// GraphQL endpoint with auth + DataLoader middleware
	gqlGroup := router.Group("/")
	gqlGroup.Use(gql.AuthMiddleware())
	gqlGroup.Use(gql.DataLoaderMiddleware(paymentRepo))
	gqlGroup.POST("/payment/graphql", gql.GraphQLHandler())
	gqlGroup.GET("/payment/graphql", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Payment GraphQL. Use POST with query."})
	})
	// Also serve on /graphql for API Gateway compatibility
	gqlGroup.POST("/graphql", gql.GraphQLHandler())

	mainSrv := &http.Server{
		Addr:         ":" + cfg.PortGraphQL,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("payment service starting",
			"port", cfg.PortGraphQL,
			"env", cfg.NodeEnv,
		)
		if err := mainSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("main server failed", "error", err)
		}
	}()

	//  16. Graceful Shutdown ─
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down payment service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	_ = mainSrv.Shutdown(shutdownCtx)
	_ = webhookSrv.Shutdown(shutdownCtx)
	grpcSrv.Stop()

	slog.Info("payment service stopped")
}

// splitBrokers splits a comma-separated broker string into a slice.
func splitBrokers(brokers string) []string {
	if brokers == "" {
		return []string{"localhost:9092"}
	}
	result := make([]string, 0)
	for _, b := range splitString(brokers, ',') {
		if b != "" {
			result = append(result, b)
		}
	}
	return result
}

func splitString(s string, sep rune) []string {
	var parts []string
	start := 0
	for i, r := range s {
		if r == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// dbPinger wraps *sql.DB to satisfy the ReadyHandler interface.
type dbPinger struct{ *sql.DB }

func (d *dbPinger) Ping() error { return d.DB.Ping() }
