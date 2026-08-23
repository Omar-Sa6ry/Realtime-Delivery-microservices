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

	sharedautomation "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/automation"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	sharedmetrics "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/metrics"
	sharednats "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/nats"
	sharedratelimiter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/ratelimiter"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/dynamodb"
	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	natsadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/nats"
	redisadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/redis"
	s3adapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/s3"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/download"
	appMedia "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/media"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/upload"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/scheduler"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/telemetry"
	gqltransport "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/graphql/dlq"
	grpctransport "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/grpc"
	pb "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/grpc/pb"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/ws"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/validation"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/workers"
	compressionWorker "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/workers/compression"
	deleteWorker "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/workers/delete"
	imageWorker "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/workers/image"
	metadataWorker "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/workers/metadata"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/workers/outbox"
	reconciliation "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/workers/reconciliation"
	scanWorker "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/workers/scan"
	videoWorker "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/workers/video"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
)

func main() {
	logger := sharedlogging.InitLogger()
	logger.Info("media-service: starting", "version", "1.0.0")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize OpenTelemetry tracing
	tp, err := telemetry.InitTracer(ctx, telemetry.Config{
		ServiceName:    "media-service",
		JaegerEndpoint: cfg.JaegerEndpoint,
		OTLPEndpoint:   cfg.OTLPEndpoint,
		Environment:    cfg.Environment,
		SampleRatio:    cfg.TraceSampleRatio,
	})
	if err != nil {
		slog.Error("Failed to initialize telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown tracer provider", "error", err)
		}
	}()

	observability.RegisterMediaMetrics()
	go func() {
		if err := sharedmetrics.StartMetricsServer(cfg.MetricsPort); err != nil {
			slog.Error("Metrics server stopped", "error", err)
		}
	}()

	slog.Info("DynamoDB config", "endpoint", cfg.DynamoDBEndpoint, "table", cfg.DynamoDBTableName, "region", cfg.AWSRegion)
	dbClient, err := dynamodb.NewClient(ctx, cfg.AWSRegion, cfg.DynamoDBEndpoint, cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey)
	if err != nil {
		slog.Error("DynamoDB client init failed", "error", err)
		os.Exit(1)
	}

	const dbRetryDelay = 5 * time.Second
	var provisionErr error
	for {
		provCtx, provCancel := context.WithTimeout(ctx, 5*time.Second)
		provisionErr = dynamodb.EnsureTableExists(provCtx, dbClient, cfg.DynamoDBTableName)
		provCancel()
		if provisionErr == nil {
			break
		}
		slog.Warn("DynamoDB not ready, retrying until available...",
			"error", provisionErr,
			"retryIn", dbRetryDelay,
		)
		select {
		case <-ctx.Done():
			slog.Error("Shutdown requested while waiting for DynamoDB", "error", ctx.Err())
			os.Exit(1)
		case <-time.After(dbRetryDelay):
		}
	}
	slog.Info("DynamoDB ready", "endpoint", cfg.DynamoDBEndpoint, "table", cfg.DynamoDBTableName)
	if err := dynamodb.EnsureTTL(ctx, dbClient, cfg.DynamoDBTableName, "TTL"); err != nil {
		slog.Warn("DynamoDB TTL could not be enabled", "error", err)
	} else {
		slog.Info("DynamoDB TTL enabled", "table", cfg.DynamoDBTableName, "attribute", "TTL")
	}

	mediaRepo := dynamodb.NewMediaRepository(dbClient, cfg.DynamoDBTableName)
	uploadRepo := dynamodb.NewUploadRepository(dbClient, cfg.DynamoDBTableName)
	outboxRepo := dynamodb.NewOutboxRepository(dbClient, cfg.DynamoDBTableName)
	quotaRepo := dynamodb.NewQuotaRepository(dbClient, cfg.DynamoDBTableName, cfg.StorageQuotaBytes, cfg.MaxConcurrentUploads)
	versionRepo := dynamodb.NewVersionRepository(dbClient, cfg.DynamoDBTableName)

	s3Client, err := s3adapter.NewClient(ctx, cfg.AWSRegion, cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.S3BucketName, cfg.S3Endpoint, cfg.S3PublicEndpoint)
	if err != nil {
		slog.Error("S3 client init failed", "error", err)
		os.Exit(1)
	}
	storage := s3adapter.NewStorageAdapter(s3Client)
	if err := s3Client.EnsureBucketExists(ctx); err != nil {
		slog.Error("S3 bucket ensure failed", "error", err)
		os.Exit(1)
	}
	if err := s3Client.EnsureBucketPolicy(ctx); err != nil {
		slog.Warn("S3 bucket policy ensure failed", "error", err)
	}
	if err := s3Client.EnsureBucketCORS(ctx); err != nil {
		slog.Warn("S3 bucket CORS ensure failed", "error", err)
	}
	if err := s3Client.EnsureBucketLifecycle(ctx, cfg.S3LifecycleAbortDays); err != nil {
		slog.Warn("S3 lifecycle could not be configured", "error", err)
	} else {
		slog.Info("S3 lifecycle enabled", "bucket", cfg.S3BucketName, "abortAfterDays", cfg.S3LifecycleAbortDays)
	}
	redisClient, err := redisadapter.NewClient(cfg.RedisAddr(), cfg.RedisPassword)
	if err != nil {
		slog.Error("Redis client init failed", "error", err)
		os.Exit(1)
	}
	cacheAdapter := redisadapter.NewCacheAdapter(redisClient)

	// NATS client for WebSocket hub
	natsClient, err := sharednats.Connect(cfg.NATSURL())
	if err != nil {
		slog.Error("NATS client init failed", "error", err)
		os.Exit(1)
	}
	defer natsClient.Close()

	// Realtime publisher for NATS progress events
	realtimePublisher := natsadapter.NewRealtimePublisher(natsClient, logger)

	// WebSocket Hub
	wsHub := ws.NewHub(natsClient, logger, cfg)
	go func() {
		if err := wsHub.Run(ctx); err != nil {
			slog.Error("WebSocket hub stopped", "error", err)
		}
	}()
	wsServer := &http.Server{
		Addr:    ":" + cfg.WSPort,
		Handler: http.HandlerFunc(wsHub.ServeWS),
	}
	go func() {
		slog.Info("WebSocket server starting", "port", cfg.WSPort)
		if err := wsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("WebSocket server error", "error", err)
			cancel()
		}
	}()

	const (
		kafkaRetryDelay  = 5 * time.Second
		kafkaDialTimeout = 5 * time.Second
	)
	if len(cfg.KafkaBrokers) > 0 {
		var kafkaErr error
		for {
			dialer := &kafkago.Dialer{Timeout: kafkaDialTimeout}
			conn, dialErr := dialer.DialContext(ctx, "tcp", cfg.KafkaBrokers[0])
			if dialErr == nil {
				_, kafkaErr = conn.Controller() // full metadata round-trip
				conn.Close()
				if kafkaErr == nil {
					break
				}
			} else {
				kafkaErr = dialErr
			}
			// Keep retrying until Kafka is truly ready so a slow boot during a
			slog.Warn("Kafka not ready, retrying until available...",
				"broker", cfg.KafkaBrokers[0],
				"error", kafkaErr,
				"retryIn", kafkaRetryDelay,
			)
			select {
			case <-ctx.Done():
				slog.Error("Shutdown requested while waiting for Kafka", "error", ctx.Err())
				os.Exit(1)
			case <-time.After(kafkaRetryDelay):
			}
		}
		slog.Info("Kafka ready", "broker", cfg.KafkaBrokers[0])
	}

	uploadRateLimiter := sharedratelimiter.NewRateLimiter(redisClient, cfg.RateLimitUploadPerMinute, time.Minute)
	downloadRateLimiter := sharedratelimiter.NewRateLimiter(redisClient, cfg.RateLimitDownloadPerMinute, time.Minute)

	producer := kafkaadapter.NewProducer(cfg.KafkaBrokers)
	defer func() {
		if err := producer.Close(); err != nil {
			slog.Error("Kafka producer close error", "error", err)
		}
	}()

	validator := validation.NewValidator(cfg.AllowedContentTypes, storage, cfg.MaxFileSizeBytes)

	scanPool := workers.NewWorkerPool("scan-worker", cfg.ScanWorkers)
	imagePool := workers.NewWorkerPool("image-worker", cfg.ImageWorkers)
	videoPool := workers.NewWorkerPool("video-worker", cfg.VideoWorkers)
	deletePool := workers.NewWorkerPool("delete-worker", cfg.DeleteWorkers)
	compressPool := workers.NewWorkerPool("compression-worker", cfg.CompressionWorkers)
	metaPool := workers.NewWorkerPool("metadata-worker", cfg.MetadataWorkers)

	createSessionUC := upload.NewCreateSessionUseCase(
		mediaRepo, uploadRepo, quotaRepo, storage, cacheAdapter,
		validator, uploadRateLimiter,
		cfg.UploadSessionTTL, cfg.S3PresignedURLExpiry, cfg.S3MultipartMinPartSize, cfg.MaxFileSizeBytes,
	)
	completeUploadUC := upload.NewCompleteUploadUseCase(
		uploadRepo, mediaRepo, outboxRepo, quotaRepo, storage, cacheAdapter, producer, validator,
	)
	abortUploadUC := upload.NewAbortUploadUseCase(uploadRepo, mediaRepo, quotaRepo, storage, cacheAdapter)
	getUploadStatusUC := upload.NewGetUploadStatusUseCase(uploadRepo, storage)

	getMediaUC := appMedia.NewGetMediaUseCase(mediaRepo, versionRepo)
	listMediaUC := appMedia.NewListMediaUseCase(mediaRepo)
	deleteMediaUC := appMedia.NewDeleteMediaUseCase(mediaRepo, outboxRepo, quotaRepo, cacheAdapter)

	getDownloadURLUC := download.NewGetDownloadUrlUseCase(
		mediaRepo, versionRepo, storage, downloadRateLimiter, cfg.S3PresignedURLExpiry,
	)

	// ── GraphQL Federation Subgraph Server
	dlqManager := dlq.NewDLQManager(producer, logger)
	renewPresignedUC := upload.NewRenewPresignedUseCase(uploadRepo, storage, cacheAdapter, cfg.S3PresignedURLExpiry)
	gqlHandler := gqltransport.NewHandler(
		createSessionUC, completeUploadUC, abortUploadUC, getUploadStatusUC,
		renewPresignedUC,
		getMediaUC, listMediaUC, deleteMediaUC, getDownloadURLUC, quotaRepo,
		dlqManager, cfg.KafkaBrokers,
	)
	gqlServer, err := gqltransport.NewServer(gqlHandler, cfg.GraphQLPort)
	if err != nil {
		slog.Error("Failed to build GraphQL server", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(grpctransport.NewServerOptions()...)
	pb.RegisterMediaServiceServer(grpcServer, grpctransport.NewServer(getDownloadURLUC, getMediaUC))

	grpcLis, err := grpctransport.NewListener(":" + cfg.GRPCPort)
	if err != nil {
		slog.Error("Failed to bind gRPC listener", "error", err)
		os.Exit(1)
	}
	go func() {
		slog.Info("Internal gRPC server starting", "port", cfg.GRPCPort)
		if err := grpcServer.Serve(grpcLis); err != nil {
			slog.Error("gRPC server error", "error", err)
			cancel()
		}
	}()

	outboxPublisher := outbox.NewPublisher(outboxRepo, producer, 5*time.Second)
	reconciler := reconciliation.NewWorker(
		uploadRepo, mediaRepo, outboxRepo, storage, cacheAdapter, producer,
		cfg.StuckUploadTimeout, cfg.StuckProcessingTimeout,
	)
	imgWorker := imageWorker.NewWorker(mediaRepo, versionRepo, storage, cacheAdapter, producer, realtimePublisher)
	vidWorker := videoWorker.NewWorker(mediaRepo, versionRepo, storage, producer, realtimePublisher)
	scnWorker := scanWorker.NewWorker(mediaRepo, storage, producer, cfg)
	delWorker := deleteWorker.NewWorker(mediaRepo, versionRepo, quotaRepo, storage, producer)
	cmpWorker := compressionWorker.NewWorker(mediaRepo, versionRepo, storage, producer)
	metaWorker := metadataWorker.NewWorker(mediaRepo, versionRepo, storage, producer)

	// Start outbox publisher in background
	go outboxPublisher.Run(ctx)

	// Scan consumer: media.upload.completed → scan worker
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicUploadCompleted,
			GroupID:    kafkaadapter.GroupScanner,
			MaxRetries: 3,
			DLQ:        producer,
		})
		defer consumer.Close()
		if err := consumer.Run(ctx, func(ctx context.Context, msg kafkago.Message) error {
			return scanPool.Submit(ctx, func(ctx context.Context) error {
				return scnWorker.Handle(ctx, msg)
			})
		}); err != nil && err != context.Canceled {
			slog.Error("Scan worker consumer stopped", "error", err)
		}
	}()

	// ── Image consumer: media.scan.completed → image worker
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicScanCompleted,
			GroupID:    kafkaadapter.GroupImageWorker,
			MaxRetries: 3,
			DLQ:        producer,
		})
		defer consumer.Close()
		if err := consumer.Run(ctx, func(ctx context.Context, msg kafkago.Message) error {
			return imagePool.Submit(ctx, func(ctx context.Context) error {
				return imgWorker.Handle(ctx, msg)
			})
		}); err != nil && err != context.Canceled {
			slog.Error("Image worker consumer stopped", "error", err)
		}
	}()

	// ── Video consumer: media.scan.completed → video worker
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicScanCompleted,
			GroupID:    kafkaadapter.GroupVideoWorker,
			MaxRetries: 3,
			DLQ:        producer,
		})
		defer consumer.Close()
		if err := consumer.Run(ctx, func(ctx context.Context, msg kafkago.Message) error {
			return videoPool.Submit(ctx, func(ctx context.Context) error {
				return vidWorker.Handle(ctx, msg)
			})
		}); err != nil && err != context.Canceled {
			slog.Error("Video worker consumer stopped", "error", err)
		}
	}()

	// ── Compression consumer: media.scan.completed → compression worker
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicScanCompleted,
			GroupID:    kafkaadapter.GroupCompressionWorker,
			MaxRetries: 3,
			DLQ:        producer,
		})
		defer consumer.Close()
		if err := consumer.Run(ctx, func(ctx context.Context, msg kafkago.Message) error {
			return compressPool.Submit(ctx, func(ctx context.Context) error {
				return cmpWorker.Handle(ctx, msg)
			})
		}); err != nil && err != context.Canceled {
			slog.Error("Compression worker consumer stopped", "error", err)
		}
	}()

	// ── Metadata consumer: media.scan.completed → metadata worker
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicScanCompleted,
			GroupID:    kafkaadapter.GroupMetadataWorker,
			MaxRetries: 3,
			DLQ:        producer,
		})
		defer consumer.Close()
		if err := consumer.Run(ctx, func(ctx context.Context, msg kafkago.Message) error {
			return metaPool.Submit(ctx, func(ctx context.Context) error {
				return metaWorker.Handle(ctx, msg)
			})
		}); err != nil && err != context.Canceled {
			slog.Error("Metadata worker consumer stopped", "error", err)
		}
	}()

	// ── Delete consumer: media.delete.requested → delete worker
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicDeleteRequested,
			GroupID:    kafkaadapter.GroupDeleteWorker,
			MaxRetries: 5,
			DLQ:        producer,
		})
		defer consumer.Close()
		if err := consumer.Run(ctx, func(ctx context.Context, msg kafkago.Message) error {
			return deletePool.Submit(ctx, func(ctx context.Context) error {
				return delWorker.Handle(ctx, msg)
			})
		}); err != nil && err != context.Canceled {
			slog.Error("Delete worker consumer stopped", "error", err)
		}
	}()

	// ── Scheduler
	sched := scheduler.NewScheduler(ctx)
	reconcileExpr := fmt.Sprintf("@every %ds", int(cfg.ReconcileInterval.Seconds()))
	sched.Add("reconciliation", reconcileExpr, reconciler.Run)
	sched.Start()
	defer sched.Stop()

	// ── System Stats Logging ──
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := sharedautomation.GetSystemStats()
				slog.Info("System stats", "details", sharedautomation.FormatStatsString(stats))
			}
		}
	}()

	// ── Signal Handling & Graceful Shutdown ───
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := gqlServer.Serve(ctx); err != nil {
			slog.Error("GraphQL server error", "error", err)
			cancel()
		}
	}()

	slog.Info("media-service: ready",
		"graphqlPort", cfg.GraphQLPort,
		"grpcPort", cfg.GRPCPort,
		"metricsPort", cfg.MetricsPort,
		"env", cfg.Environment,
	)

	<-sigCh
	slog.Info("media-service: shutdown signal received, draining...")
	cancel()

	// Gracefully stop the internal gRPC server
	grpcServer.GracefulStop()

	// Gracefully stop WebSocket server
	wsShutdownCtx, wsShutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer wsShutdownCancel()
	if err := wsServer.Shutdown(wsShutdownCtx); err != nil {
		slog.Error("WebSocket server shutdown error", "error", err)
	}

	// Wait for worker pools to drain
	scanPool.Shutdown()
	imagePool.Shutdown()
	videoPool.Shutdown()
	deletePool.Shutdown()
	compressPool.Shutdown()
	metaPool.Shutdown()

	slog.Info("media-service: shutdown complete")
}
