package graphql

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

// Server ...
type Server struct {
	schema *Schema
	httpServer *http.Server
	stopChan  chan struct{}
}

// NewServer creates a new GraphQL server.
func NewServer(sdl string) *Server {
	return &Server{
		schema: &Schema{sdl: sdl},
	}
}

// Start starts the GraphQL server.
func (s *Server) Start(addr string) error {
	log.Printf("Starting GraphQL server on %s", addr)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: Handler(s.schema),
	}

	go func() {
		log.Printf("GraphQL server listening on :%s", addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("GraphQL server error: %v", err)
		}
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	<-quit

	log.Println("Shutting down GraphQL server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("GraphQL server shutdown error: %v", err)
	}

	close(s.stopChan)
	log.Println("GraphQL server stopped.")
	return nil
}

// Stop stops the HTTP server.
func (s *Server) Stop() {
	if s.httpServer != nil {
		s.httpServer.Shutdown(context.Background())
	}
	close(s.stopChan)
}