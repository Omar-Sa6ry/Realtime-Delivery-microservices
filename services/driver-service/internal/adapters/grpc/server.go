package grpc

import (
	"log"
	"net"

	"google.golang.org/grpc"
)

// GRPCServer implements the gRPC service for driver operations.
type GRPCServer struct {
	server *grpc.Server
}

// NewGRPCServer creates a new GRPCServer.
func NewGRPCServer() *GRPCServer {
	return &GRPCServer{
		server: grpc.NewServer(),
	}
}

// Start starts the gRPC server.
func (s *GRPCServer) Start(l net.Listener) error {
	log.Printf("Starting gRPC server on %v", l.Addr())
	return s.server.Serve(l)
}

// Stop stops the gRPC server.
func (s *GRPCServer) Stop() {
	s.server.GracefulStop()
}