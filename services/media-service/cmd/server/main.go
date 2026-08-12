package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	sharedautomation "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/automation"
	sharedlogging "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/logging"
	sharedmetrics "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/metrics"
	sharedratelimiter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/packages/go/ratelimiter"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/dynamodb"
	kafkaadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/kafka"
	redisadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/redis"
	s3adapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/s3"
	wsadapter "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/adapters/ws"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/download"
	appMedia "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/media"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/application/upload"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/observability"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/scheduler"
	grpctransport "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/grpc"
	wstransport "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/transport/websocket"
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
)

func main() {
	// ── Structured Logger (shared package) ────────────────────────────────────
	logger := sharedlogging.InitLogger()
	logger.Info("media-service: starting", "version", "1.0.0")

	// ── Configuration ─────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// ── Root Context with Cancellation ────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Prometheus Metrics (shared + media-specific) ───────────────────────────
	observability.RegisterMediaMetrics()
	go func() {
		if err := sharedmetrics.StartMetricsServer(cfg.MetricsPort); err != nil {
			slog.Error("Metrics server stopped", "error", err)
		}
	}()

	// ── DynamoDB Client & Table Provisioning ──────────────────────────────────
	dbClient, err := dynamodb.NewClient(ctx, cfg.AWSRegion, cfg.DynamoDBEndpoint, cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey)
	if err != nil {
		slog.Error("DynamoDB client init failed", "error", err)
		os.Exit(1)
	}
	if err := dynamodb.EnsureTableExists(ctx, dbClient, cfg.DynamoDBTableName); err != nil {
		slog.Error("DynamoDB table provisioning failed", "error", err)
		os.Exit(1)
	}
	slog.Info("DynamoDB ready", "table", cfg.DynamoDBTableName)

	// ── Repositories ──────────────────────────────────────────────────────────
	mediaRepo := dynamodb.NewMediaRepository(dbClient, cfg.DynamoDBTableName)
	uploadRepo := dynamodb.NewUploadRepository(dbClient, cfg.DynamoDBTableName)
	outboxRepo := dynamodb.NewOutboxRepository(dbClient, cfg.DynamoDBTableName)
	quotaRepo := dynamodb.NewQuotaRepository(dbClient, cfg.DynamoDBTableName, cfg.StorageQuotaBytes, cfg.MaxConcurrentUploads)
	versionRepo := dynamodb.NewVersionRepository(dbClient, cfg.DynamoDBTableName)

	// ── S3 Client & Storage Adapter ───────────────────────────────────────────
	s3Client, err := s3adapter.NewClient(ctx, cfg.AWSRegion, cfg.AWSAccessKeyID, cfg.AWSSecretAccessKey, cfg.S3BucketName)
	if err != nil {
		slog.Error("S3 client init failed", "error", err)
		os.Exit(1)
	}
	storage := s3adapter.NewStorageAdapter(s3Client)
	slog.Info("S3 ready", "bucket", cfg.S3BucketName)

	// ── Redis Client ──────────────────────────────────────────────────────────
	redisClient, err := redisadapter.NewClient(cfg.RedisAddr(), cfg.RedisPassword)
	if err != nil {
		slog.Error("Redis client init failed", "error", err)
		os.Exit(1)
	}
	// Health check via shared automation package
	redisHealth := sharedautomation.CheckRedis(ctx, redisClient)
	if redisHealth.Status != "UP" {
		slog.Error("Redis health check failed", "message", redisHealth.Message)
		os.Exit(1)
	}
	cacheAdapter := redisadapter.NewCacheAdapter(redisClient)
	slog.Info("Redis ready", "addr", cfg.RedisAddr())

	// ── Rate Limiter (shared package) ─────────────────────────────────────────
	uploadRateLimiter := sharedratelimiter.NewRateLimiter(redisClient, cfg.RateLimitUploadPerMinute, time.Minute)
	downloadRateLimiter := sharedratelimiter.NewRateLimiter(redisClient, cfg.RateLimitDownloadPerMinute, time.Minute)

	// ── Kafka Producer ────────────────────────────────────────────────────────
	producer := kafkaadapter.NewProducer(cfg.KafkaBrokers)
	defer func() {
		if err := producer.Close(); err != nil {
			slog.Error("Kafka producer close error", "error", err)
		}
	}()

	// ── Validator ─────────────────────────────────────────────────────────────
	validator := validation.NewValidator(cfg.AllowedContentTypes, storage, cfg.MaxFileSizeBytes)

	// ── Worker Pools ──────────────────────────────────────────────────────────
	scanPool        := workers.NewWorkerPool("scan-worker",        cfg.ScanWorkers)
	imagePool       := workers.NewWorkerPool("image-worker",       cfg.ImageWorkers)
	videoPool       := workers.NewWorkerPool("video-worker",       cfg.VideoWorkers)
	deletePool      := workers.NewWorkerPool("delete-worker",      cfg.DeleteWorkers)
	compressPool    := workers.NewWorkerPool("compression-worker", cfg.CompressionWorkers)
	metaPool        := workers.NewWorkerPool("metadata-worker",    cfg.MetadataWorkers)

	// ── WebSocket Hub + Notifier ───────────────────────────────────────
	wsHub     := wstransport.NewHub(redisClient)
	notifier  := wsadapter.NewPubSubNotifier(redisClient)
	wsServer  := wstransport.NewServer(":"+cfg.WSPort, wsHub)

	go func() {
		slog.Info("WebSocket server starting", "port", cfg.WSPort)
		if err := wsServer.ListenAndServe(); err != nil {
			slog.Error("WebSocket server error", "error", err)
			cancel()
		}
	}()

	// ── Application Use Cases ─────────────────────────────────────────────────
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

	// ── gRPC Server ───────────────────────────────────────────────────────────
	grpcHandler := grpctransport.NewHandler(
		createSessionUC, completeUploadUC, abortUploadUC, getUploadStatusUC,
		getMediaUC, listMediaUC, deleteMediaUC, getDownloadURLUC, quotaRepo,
	)
	grpcServer := grpctransport.NewServer(grpcHandler, uploadRateLimiter, cfg.GRPCPort)

	// ── Background Workers ────────────────────────────────────────────────────
	outboxPublisher := outbox.NewPublisher(outboxRepo, producer, 5*time.Second)
	reconciler := reconciliation.NewWorker(
		uploadRepo, mediaRepo, outboxRepo, storage, cacheAdapter, producer,
		cfg.StuckUploadTimeout, cfg.StuckProcessingTimeout,
	)
	imgWorker   := imageWorker.NewWorker(mediaRepo, versionRepo, storage, cacheAdapter, producer)
	vidWorker   := videoWorker.NewWorker(mediaRepo, versionRepo, storage, producer)
	scnWorker   := scanWorker.NewWorker(mediaRepo, storage, producer)
	delWorker   := deleteWorker.NewWorker(mediaRepo, versionRepo, quotaRepo, storage, producer)
	cmpWorker   := compressionWorker.NewWorker(mediaRepo, versionRepo, storage, producer, notifier)
	metaWorker  := metadataWorker.NewWorker(mediaRepo, versionRepo, storage, producer, notifier)

	// Start outbox publisher in background
	go outboxPublisher.Run(ctx)

	// ── Scan consumer: media.upload.completed → scan worker ───────────────────
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicUploadCompleted,
			GroupID:    kafkaadapter.GroupScanner,
			MaxRetries: 3,
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

	// ── Image consumer: media.scan.completed → image worker ───────────────────
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicScanCompleted,
			GroupID:    kafkaadapter.GroupImageWorker,
			MaxRetries: 3,
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

	// ── Video consumer: media.scan.completed → video worker ───────────────────
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicScanCompleted,
			GroupID:    kafkaadapter.GroupVideoWorker,
			MaxRetries: 3,
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

	// ── Compression consumer: media.scan.completed → compression worker ───────
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicScanCompleted,
			GroupID:    kafkaadapter.GroupCompressionWorker,
			MaxRetries: 3,
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

	// ── Metadata consumer: media.scan.completed → metadata worker ──────────
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicScanCompleted,
			GroupID:    kafkaadapter.GroupMetadataWorker,
			MaxRetries: 3,
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

	// ── Delete consumer: media.delete.requested → delete worker ──────────────
	go func() {
		consumer := kafkaadapter.NewConsumer(kafkaadapter.ConsumerConfig{
			Brokers:    cfg.KafkaBrokers,
			Topic:      kafkaadapter.TopicDeleteRequested,
			GroupID:    kafkaadapter.GroupDeleteWorker,
			MaxRetries: 5,
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

	// ── Scheduler ─────────────────────────────────────────────────────────────
	sched := scheduler.NewScheduler(ctx)
	reconcileExpr := fmt.Sprintf("@every %ds", int(cfg.ReconcileInterval.Seconds()))
	sched.Add("reconciliation", reconcileExpr, reconciler.Run)
	sched.Start()
	defer sched.Stop()

	// ── System Stats Logging ──────────────────────────────────────────────────
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

	// ── Signal Handling & Graceful Shutdown ────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := grpcServer.Serve(ctx); err != nil {
			slog.Error("gRPC server error", "error", err)
			cancel()
		}
	}()

	slog.Info("media-service: ready",
		"grpcPort", cfg.GRPCPort,
		"wsPort", cfg.WSPort,
		"metricsPort", cfg.MetricsPort,
		"env", cfg.Environment,
	)

	<-sigCh
	slog.Info("media-service: shutdown signal received, draining...")
	cancel()

	// Wait for worker pools to drain
	scanPool.Shutdown()
	imagePool.Shutdown()
	videoPool.Shutdown()
	deletePool.Shutdown()
	compressPool.Shutdown()
	metaPool.Shutdown()

	// Shutdown WebSocket server
	_ = wsServer.Shutdown(ctx)
	_ = notifier.Close()

	slog.Info("media-service: shutdown complete")
}
