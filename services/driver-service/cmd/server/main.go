package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/config"
)

const driverSubgraphSDL = `extend schema @link(url: "https://specs.apollo.dev/federation/v2.3", import: ["@key", "@shareable"])

type Driver @key(fields: "id") {
  id: ID!
  userId: String!
  name: String
  status: String!
  vehicleType: String
  rating: Float
}

type Query {
  _service: _Service!
  driver(id: ID!): Driver
}

type _Service {
  sdl: String!
}`

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func graphqlHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"__typename":"Query"}}`))
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
				"_service": map[string]interface{}{
					"sdl": driverSubgraphSDL,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"__typename": "Query",
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func main() {
	cfg := config.Load()
	log.Printf("Starting Driver Service - GraphQL port: %s, gRPC port: %s, Metrics port: %s", cfg.PortGraphQL, cfg.PortGRPC, cfg.PortMetrics)

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", healthHandler)
	mux.HandleFunc("/health/ready", healthHandler)
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/driver/graphql", graphqlHandler)
	mux.HandleFunc("/graphql", graphqlHandler)

	grpcHealthServer := &http.Server{
		Addr:    ":" + cfg.PortGRPC,
		Handler: mux,
	}

	gqlServer := &http.Server{
		Addr:    ":" + cfg.PortGraphQL,
		Handler: mux,
	}

	go func() {
		log.Printf("gRPC/Health server listening on :%s", cfg.PortGRPC)
		if err := grpcHealthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	go func() {
		log.Printf("GraphQL server listening on :%s", cfg.PortGraphQL)
		if err := gqlServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("GraphQL server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Driver Service gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = grpcHealthServer.Shutdown(ctx)
	_ = gqlServer.Shutdown(ctx)
	log.Println("Driver Service stopped.")
}