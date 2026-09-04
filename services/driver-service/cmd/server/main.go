package main

import (
	"fmt"
	"log"
	"net"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/driver-service/internal/config"
)

func main() {
	cfg := config.Load()
	log.Printf("Config loaded - GraphQL port: %s, gRPC port: %s", cfg.PortGraphQL, cfg.PortGRPC)

	// Just verify we can listen
	lis, err := net.Listen("tcp", ":"+cfg.PortGRPC)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	fmt.Printf("Listening on %s\n", cfg.PortGRPC)
	_ = lis
}