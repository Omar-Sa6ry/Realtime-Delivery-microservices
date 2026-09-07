package main

import (
	"context"
	"encoding/json"
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
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/workers"
)

// driverSubgraphSDL is the GraphQL SDL for the driver subgraph (Apollo Federation v2).
const driverSubgraphSDL = `extend schema
  @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])

type Driver @key(fields: "id") {
  id: ID!
  userId: String!
  status: String!
  vehicleType: String
  plateNumber: String
  capacityKg: Int
  capabilities: [String!]
  serviceArea: String
  rating: Float
  createdAt: String
  updatedAt: String
}

type Assignment {
  id: ID!
  deliveryId: String!
  driverId: String!
  status: String!
  attemptNumber: Int
  offeredAt: String
  expiresAt: String
  acceptedAt: String
  rejectedAt: String
  completedAt: String
  createdAt: String
  updatedAt: String
}

type DriverStatus {
  driverId: String!
  status: String!
  hasActiveAssignment: Boolean!
  activeDeliveryId: String
  lastSeenAt: String
}

type NearbyDriverItem {
  driverId: String!
  distanceMeters: Float!
  status: String!
  vehicleType: String
  latitude: Float!
  longitude: Float!
}

type NearbyDriversResult {
  items: [NearbyDriverItem!]!
  total: Int!
}

type DispatchAttemptItem {
  id: ID!
  deliveryId: String!
  driverId: String!
  distanceMeters: Float!
  attemptNumber: Int!
  result: String!
  reason: String
  createdAt: String!
}

type DispatchAttemptsResult {
  items: [DispatchAttemptItem!]!
  total: Int!
}

type NearbyDriversInput {
  latitude: Float!
  longitude: Float!
  radiusKm: Float!
  vehicleType: String
  limit: Int
}

input RegisterDriverInput {
  userId: String!
  vehicleType: String!
  plateNumber: String!
  capacityKg: Int!
  capabilities: [String!]
  serviceArea: String
}

input UpdateDriverProfileInput {
  vehicleType: String
  plateNumber: String
  capacityKg: Int
  capabilities: [String!]
  serviceArea: String
}

type Query {
  _service: _Service!
  driverServiceInfo: DriverServiceInfoResponse
  driver(id: ID!): Driver
  myDriverProfile: Driver
  driverActiveAssignment(driverId: ID!): Assignment
  driverStatus(driverId: ID!): DriverStatus
  nearbyDrivers(input: NearbyDriversInput!): NearbyDriversResult
  assignment(id: ID!): Assignment
  dispatchAttempts(deliveryId: ID!): DispatchAttemptsResult
}

type Mutation {
  goOnline(idempotencyKey: String!): Driver
  goOffline(idempotencyKey: String!): Driver
  acceptAssignment(assignmentId: ID!, idempotencyKey: String!): Assignment
  rejectAssignment(assignmentId: ID!, reason: String, idempotencyKey: String!): Assignment
  registerDriver(input: RegisterDriverInput!): Driver
  updateDriverProfile(driverId: ID!, input: UpdateDriverProfileInput!): Driver
  suspendDriver(driverId: ID!, reason: String!): Driver
  activateDriver(driverId: ID!): Driver
}

type Subscription {
  driverLocationUpdated: Driver
  driverAssignmentOffered(assignmentId: ID!): Assignment
  driverAssignmentAccepted(assignmentId: ID!): Assignment
  driverStatusUpdated(driverId: ID!): DriverStatus
}

type _Service {
  sdl: String!
}

type DriverServiceInfoResponse {
  success: Boolean!
  statusCode: Int!
  message: String!
  timeStamp: String!
  data: DriverServiceInfo
}

type DriverServiceInfo {
  name: String!
  version: String!
  status: String!
}`

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","service":"driver-service"}`))
}

func graphqlHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method == http.MethodGet {
		resp := map[string]interface{}{"data": map[string]interface{}{"__typename": "Query"}}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if strings.Contains(req.Query, "_service") {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"_service": map[string]interface{}{"sdl": driverSubgraphSDL},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	if strings.Contains(req.Query, "driverServiceInfo") {
		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"driverServiceInfo": map[string]interface{}{
					"success":    true,
					"statusCode": 200,
					"message":    "Driver service is running",
					"timeStamp":  time.Now().UTC().Format(time.RFC3339),
					"data": map[string]interface{}{
						"name":    "driver-service",
						"version": "1.0.0",
						"status":  "healthy",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp := map[string]interface{}{
		"data": map[string]interface{}{"__typename": "Query"},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

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
	_ = idempotencyRepo // used by commands

	// ─── Redis ────────────────────────────────────────────────────────────────
	redisAddr := cfg.RedisHost + ":" + cfg.RedisPort
	rdb := redisclient.NewClient(&redisclient.Options{Addr: redisAddr})
	geoStore := adapterredis.NewGeoStore(rdb)
	lockManager := adapterredis.NewLockManager(rdb)

	// ─── Kafka Publisher ──────────────────────────────────────────────────────
	kafkaBrokers := strings.Split(cfg.KafkaBrokers, ",")
	kafkaPub := adapterkafka.NewKafkaPublisher(kafkaBrokers, "driver-events")

	// ─── NATS ─────────────────────────────────────────────────────────────────
	// NATS publisher (optional — best effort, no fatal if unavailable)
	_ = geoStore  // used by dispatch service
	_ = kafkaPub  // used via EventPublisher wrapper

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

	// ─── GraphQL HTTP Server ──────────────────────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", healthHandler)
	mux.HandleFunc("/health/ready", healthHandler)
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/driver/graphql", graphqlHandler)
	mux.HandleFunc("/graphql", graphqlHandler)

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