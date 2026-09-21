package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/adapters/clickhouse"
	grpcadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/adapters/grpc"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/adapters/idempotency"
	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/adapters/kafka"
	natsadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/adapters/nats"
	redisadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/adapters/redis"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/analytics"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion/handlers"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/reconciliation"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/config"
	gql "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/workers"
)

func main() {
	logger := observability.Init()
	_ = logger

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	analyticsMetrics := observability.NewMetrics()
	tracer, err := observability.NewTracer(observability.TracerConfig{
		ServiceName:    "analytics-service",
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

	cache := redisadapter.NewCache(cfg.RedisHost, cfg.RedisPort)
	defer cache.Close()

	brokers := splitBrokers(cfg.KafkaBrokers)
	if err := kafkaadapter.EnsureAnalyticsTopics(brokers, cfg.DLQTopic, 6, 1); err != nil {
		slog.Warn("kafka topics setup failed (non-fatal)", "error", err)
	}
	dlqPublisher := kafkaadapter.NewDLQPublisher(brokers, cfg.DLQTopic, cfg.KafkaGroupID)
	defer dlqPublisher.Close()

	chWriter := clickhouse.NewMetricsWriter(clickhouse.NewWriter(chClient), analyticsMetrics)
	idempotencyStore := idempotency.NewStore(chClient.DB())

	batchWriter := ingestion.NewBatchWriter(
		chWriter, idempotencyStore,
		cfg.BatchSize, time.Duration(cfg.BatchFlushMS)*time.Millisecond,
	)
	ingestCtx, stopIngest := context.WithCancel(context.Background())
	batchWriter.Start(ingestCtx)

	router := ingestion.NewRouter()
	router.Register("delivery", &handlers.DeliveryHandler{})
	router.Register("driver", &handlers.DriverHandler{})
	router.Register("payment", &handlers.PaymentHandler{})
	router.Register("notification", &handlers.NotificationHandler{})
	pipeline := ingestion.NewIngestionPipeline(
		ingestion.NewValidator(),
		ingestion.NewDeduplicator(idempotencyStore),
		router,
		batchWriter,
	)

	consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
		Brokers:    brokers,
		Topics:     kafkaadapter.AnalyticsTopics(),
		GroupID:    cfg.KafkaGroupID,
		MaxRetries: cfg.MaxRetryAttempts,
		DLQ:        dlqPublisher,
	})
	go func() {
		slog.Info("kafka consumer starting", "topics", kafkaadapter.AnalyticsTopics(), "group", cfg.KafkaGroupID)
		if err := consumer.Run(ingestCtx, bridgeToPipeline(pipeline, analyticsMetrics)); err != nil {
			slog.Error("kafka consumer failed", "error", err)
		}
	}()
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := batchWriter.Stop(flushCtx); err != nil {
			slog.Error("final batch flush failed", "error", err)
		}
	}()
	defer consumer.Close()
	defer stopIngest()

	queryRepo := clickhouse.NewQueryRepository(chClient)
	cacheTTL := time.Duration(cfg.CacheTTLSeconds) * time.Second

	platformOverviewSvc := analytics.NewPlatformOverviewService(queryRepo, cache, cacheTTL)
	deliveryAnalyticsSvc := analytics.NewDeliveryAnalyticsService(queryRepo, cache, cacheTTL)
	driverAnalyticsSvc := analytics.NewDriverAnalyticsService(queryRepo, cache, cacheTTL)
	paymentAnalyticsSvc := analytics.NewPaymentAnalyticsService(queryRepo, cache, cacheTTL)

	resolver := gql.NewResolver(
		platformOverviewSvc,
		deliveryAnalyticsSvc,
		driverAnalyticsSvc,
		paymentAnalyticsSvc,
		queryRepo,
	)

	// --- Internal gRPC Server for mesh communication ---
	grpcSrv := grpcadapter.NewServer(cfg.PortGRPC, platformOverviewSvc, driverAnalyticsSvc)
	go func() {
		slog.Info("analytics gRPC server starting", "port", cfg.PortGRPC)
		if err := grpcSrv.Start(); err != nil {
			slog.Error("analytics gRPC server failed", "error", err)
		}
	}()
	defer grpcSrv.Stop()

	// --- NATS Publisher for live transient ticker to Realtime Service ---
	natsPublisher, err := natsadapter.NewPublisher(cfg.NATSURL)
	if err != nil {
		slog.Warn("nats publisher unavailable (non-fatal)", "error", err)
	}
	if natsPublisher != nil {
		defer natsPublisher.Close()
	}

	pool := workers.NewPool()
	reconciliationSvc := reconciliation.NewService(queryRepo, reconciliation.Config{WindowHours: 1})
	pool.Register("reconciliation", time.Duration(cfg.ReconcileIntervalSec)*time.Second,
		workers.NewReconciliationWorker(reconciliationSvc).Run)

	if natsPublisher != nil {
		pool.Register("live_ticker", 10*time.Second, func(ctx context.Context) {
			now := time.Now().UTC()
			overview, err := platformOverviewSvc.GetPlatformOverview(ctx, ports.TimeRange{
				From:        now.Add(-1 * time.Hour),
				To:          now,
				Granularity: ports.GranularityHour,
			}, "nats")
			if err != nil {
				return
			}
			rate := 0.0
			if overview.TotalDeliveries > 0 {
				rate = float64(overview.CompletedDeliveries) / float64(overview.TotalDeliveries)
			}
			_ = natsPublisher.PublishRealtimeMetrics(natsadapter.RealtimeMetricPayload{
				Timestamp:      now.Format(time.RFC3339),
				ActiveEvents:   overview.TotalDeliveries,
				CompletionRate: rate,
				FreshnessSec:   analyticsMetrics.GetFreshnessSeconds(),
			})
		})
	}

	workerCtx, stopWorkers := context.WithCancel(context.Background())
	defer stopWorkers()
	pool.Start(workerCtx)
	defer pool.Stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", gql.HealthLiveHandler)
	mux.HandleFunc("/health/ready", gql.HealthReadyHandler(chClient.Ping))
	mux.Handle("/metrics", promhttp.Handler())

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
	grpcSrv.Stop()
	slog.Info("analytics service stopped")
}

func splitBrokers(brokers string) []string {
	if strings.TrimSpace(brokers) == "" {
		return []string{"localhost:9092"}
	}
	parts := strings.Split(brokers, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

type envelopeMeta struct {
	EventType  string          `json:"eventType"`
	OccurredAt json.RawMessage `json:"occurredAt"`
}

func extractMeta(data []byte) (eventType string, occurredAt time.Time) {
	var meta envelopeMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return "unknown", time.Time{}
	}
	s := strings.TrimSpace(string(meta.OccurredAt))
	if strings.HasPrefix(s, `"`) {
		var str string
		if err := json.Unmarshal(meta.OccurredAt, &str); err == nil {
			if tm, err := time.Parse(time.RFC3339, strings.TrimSpace(str)); err == nil {
				return meta.EventType, tm
			}
		}
		return meta.EventType, time.Time{}
	}
	var num json.Number
	if err := json.Unmarshal(meta.OccurredAt, &num); err == nil {
		if ms, err := num.Int64(); err == nil {
			return meta.EventType, time.UnixMilli(ms).UTC()
		}
	}
	return meta.EventType, time.Time{}
}

func bridgeToPipeline(pipeline *ingestion.IngestionPipeline, metrics *observability.Metrics) kafkaadapter.Handler {
	return func(ctx context.Context, topic string, msg kafkago.Message) error {
		start := time.Now()
		metrics.RecordConsumed(topic)
		eventType, occurredAt := extractMeta(msg.Value)
		if eventType == "" {
			eventType = "unknown"
		}

		outcome, err := pipeline.Process(ctx, topic, int32(msg.Partition), msg.Offset, msg.Value)
		latencyMs := float64(time.Since(start).Milliseconds())
		if err != nil {
			metrics.RecordFailed(eventType)
			observability.RecordError(ctx, err)
			return err
		}
		if !occurredAt.IsZero() {
			metrics.SetFreshness(time.Since(occurredAt).Seconds())
		}
		if outcome == ingestion.OutcomeDuplicate {
			metrics.RecordDuplicate(topic)
			metrics.RecordProcessed(eventType, "duplicate", latencyMs)
		} else {
			metrics.RecordProcessed(eventType, "success", latencyMs)
		}
		return nil
	}
}
