package grpc

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ErrorMapper interface {
	MapError(err error) error
}

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

type UnauthorizedErrorMapper struct{}

func (m *UnauthorizedErrorMapper) MapError(err error) error {
	if errors.Is(err, domain.ErrUnauthorized) {
		return status.Error(codes.PermissionDenied, "access denied: resource ownership mismatch")
	}
	return nil
}

type NotFoundErrorMapper struct{}

func (m *NotFoundErrorMapper) MapError(err error) error {
	if errors.Is(err, domain.ErrMediaNotFound) {
		return status.Error(codes.NotFound, "media not found")
	}
	return nil
}

type QuarantinedErrorMapper struct{}

func (m *QuarantinedErrorMapper) MapError(err error) error {
	if errors.Is(err, domain.ErrMediaQuarantined) {
		return status.Error(codes.FailedPrecondition, "media is quarantined")
	}
	return nil
}

type RateLimitErrorMapper struct{}

func (m *RateLimitErrorMapper) MapError(err error) error {
	if errors.Is(err, domain.ErrRateLimitExceeded) {
		return status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}
	return nil
}

func BuildErrorMapperRegistry() *errorMapperRegistry {
	registry := newErrorMapperRegistry()
	registry.register(&UnauthorizedErrorMapper{})
	registry.register(&NotFoundErrorMapper{})
	registry.register(&QuarantinedErrorMapper{})
	registry.register(&RateLimitErrorMapper{})
	return registry
}

func toGRPCStatus(err error, registry *errorMapperRegistry) error {
	if mapped := registry.mapError(err); mapped != nil {
		return mapped
	}
	slog.Error("grpc media use-case error", "error", err)
	return status.Error(codes.Internal, fmt.Sprintf("internal error: %v", err))
}