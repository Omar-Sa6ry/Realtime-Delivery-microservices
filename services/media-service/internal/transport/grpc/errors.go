package grpc

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorMapper maps domain errors to gRPC status codes.
type ErrorMapper interface {
	// MapError maps a domain error to a gRPC status error.
	// Returns nil if the error is not handled by this mapper.
	MapError(err error) error
}

// errorMapperRegistry holds all registered error mappers.
type errorMapperRegistry struct {
	mappers []ErrorMapper
}

func newErrorMapperRegistry() *errorMapperRegistry {
	return &errorMapperRegistry{}
}

func (r *errorMapperRegistry) register(m ErrorMapper) {
	r.mappers = append(r.mappers, m)
}

func (r *errorMapperRegistry) mapError(err error) error {
	for _, m := range r.mappers {
		if mapped := m.MapError(err); mapped != nil {
			return mapped
		}
	}
	return nil
}

// UnauthorizedErrorMapper maps unauthorized errors.
type UnauthorizedErrorMapper struct{}

func (m *UnauthorizedErrorMapper) MapError(err error) error {
	if errors.Is(err, domain.ErrUnauthorized) {
		return status.Error(codes.PermissionDenied, "access denied: resource ownership mismatch")
	}
	return nil
}

// NotFoundErrorMapper maps not found errors.
type NotFoundErrorMapper struct{}

func (m *NotFoundErrorMapper) MapError(err error) error {
	if errors.Is(err, domain.ErrMediaNotFound) {
		return status.Error(codes.NotFound, "media not found")
	}
	return nil
}

// QuarantinedErrorMapper maps quarantined errors.
type QuarantinedErrorMapper struct{}

func (m *QuarantinedErrorMapper) MapError(err error) error {
	if errors.Is(err, domain.ErrMediaQuarantined) {
		return status.Error(codes.FailedPrecondition, "media is quarantined")
	}
	return nil
}

// RateLimitErrorMapper maps rate limit errors.
type RateLimitErrorMapper struct{}

func (m *RateLimitErrorMapper) MapError(err error) error {
	if errors.Is(err, domain.ErrRateLimitExceeded) {
		return status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}
	return nil
}

// BuildErrorMapperRegistry creates and populates the error mapper registry.
func BuildErrorMapperRegistry() *errorMapperRegistry {
	registry := newErrorMapperRegistry()
	registry.register(&UnauthorizedErrorMapper{})
	registry.register(&NotFoundErrorMapper{})
	registry.register(&QuarantinedErrorMapper{})
	registry.register(&RateLimitErrorMapper{})
	return registry
}

// toGRPCStatus converts a domain error to a gRPC status error using the mapper registry.
func toGRPCStatus(err error, registry *errorMapperRegistry) error {
	if mapped := registry.mapError(err); mapped != nil {
		return mapped
	}
	slog.Error("grpc media use-case error", "error", err)
	return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
}