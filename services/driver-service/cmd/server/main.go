package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	adaptergrpc "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/grpc"
	adapterkafka "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/kafka"
	adaptermongo "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/mongodb"
	adapterredis "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/adapters/redis"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/application/services"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/config"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/domain"
	internalgql "github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/graphql"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/workers"
)

func main() {
	cfg := config.Load()
	log.Printf("Driver Service starting — GraphQL: %s | gRPC: %s | Metrics: %s",
		cfg.PortGraphQL, cfg.PortGRPC, cfg.PortMetrics)

	// ─── MongoDB ──────────────────────────────────────────────────────────────
	mongoCtx, mongoCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer mongoCancel()
	mongoClient, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoDBURI))
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	if err = mongoClient.Ping(mongoCtx, nil); err != nil {
		log.Printf("WARNING: MongoDB ping failed: %v — service will retry on demand", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mongoClient.Disconnect(ctx)
	}()

	// ─── Repositories ─────────────────────────────────────────────────────────
	driverRepo := adaptermongo.NewDriverRepository(mongoClient, cfg.MongoDBDatabase, "drivers")
	assignmentRepo := adaptermongo.NewAssignmentRepository(mongoClient, cfg.MongoDBDatabase, "assignments")
	idempotencyRepo := adaptermongo.NewIdempotencyRepository(mongoClient, cfg.MongoDBDatabase, "idempotency")
	_ = idempotencyRepo

	// ─── Redis ────────────────────────────────────────────────────────────────
	redisAddr := cfg.RedisHost + ":" + cfg.RedisPort
	rdb := redisclient.NewClient(&redisclient.Options{Addr: redisAddr})
	geoStore := adapterredis.NewGeoStore(rdb)
	lockManager := adapterredis.NewLockManager(rdb)

	// ─── Kafka Publisher ──────────────────────────────────────────────────────
	kafkaBrokers := strings.Split(cfg.KafkaBrokers, ",")
	kafkaPub := adapterkafka.NewKafkaPublisher(kafkaBrokers, "driver-events")

	// ─── Application Service ──────────────────────────────────────────────────
	dispatchPolicy := domain.NewDispatchPolicy()
	eventPublisher := adapterkafka.NewEventPublisherAdapter(kafkaPub)
	dispatchSvc := services.NewDispatchService(
		driverRepo,
		assignmentRepo,
		geoStore,
		lockManager,
		eventPublisher,
		dispatchPolicy,
	)

	// ─── Background Workers ───────────────────────────────────────────────────
	wp := workers.NewWorkerPool(3)
	expiryWorker := workers.NewAssignmentExpiryWorker(assignmentRepo, dispatchSvc)
	reconcileWorker := workers.NewReconciliationWorker(driverRepo, assignmentRepo)
	heartbeatWorker := workers.NewHeartbeatMonitor(driverRepo)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	wp.Submit(func() { expiryWorker.Run(workerCtx, 15*time.Second) })
	wp.Submit(func() { reconcileWorker.Run(workerCtx, 60*time.Second) })
	wp.Submit(func() { heartbeatWorker.Run(workerCtx, 30*time.Second) })

	// ─── gRPC Server ──────────────────────────────────────────────────────────
	grpcListener, err := net.Listen("tcp", ":"+cfg.PortGRPC)
	if err != nil {
		log.Fatalf("failed to bind gRPC port %s: %v", cfg.PortGRPC, err)
	}
	grpcSrv := adaptergrpc.NewGRPCServer(dispatchSvc, driverRepo)
	go func() {
		if err := grpcSrv.Start(grpcListener); err != nil {
			log.Printf("gRPC server stopped: %v", err)
		}
	}()

	// ─── GraphQL & Health HTTP Server ─────────────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", internalgql.HealthHandler)
	mux.HandleFunc("/health/ready", internalgql.HealthHandler)
	mux.HandleFunc("/healthz", internalgql.HealthHandler)
	mux.HandleFunc("/driver/graphql", internalgql.Handler)
	mux.HandleFunc("/graphql", internalgql.Handler)

	gqlServer := &http.Server{
		Addr:         ":" + cfg.PortGraphQL,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Printf("GraphQL server listening on :%s", cfg.PortGraphQL)
		if err := gqlServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("GraphQL server error: %v", err)
		}
	}()

	// ─── Graceful Shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Driver Service gracefully...")
	grpcSrv.Stop()
	workerCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = gqlServer.Shutdown(shutdownCtx)

	log.Println("Driver Service stopped.")
}