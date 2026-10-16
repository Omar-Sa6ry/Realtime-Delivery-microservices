package grpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorMapperMapError(t *testing.T) {
	tests := []struct {
		name     string
		mapper   ErrorMapper
		err      error
		wantCode codes.Code
		wantNil  bool
	}{
		{
			name:     "unauthorized mapper matches ErrUnauthorized",
			mapper:   &UnauthorizedErrorMapper{},
			err:      domain.ErrUnauthorized,
			wantCode: codes.PermissionDenied,
		},
		{
			name:     "unauthorized mapper matches wrapped error",
			mapper:   &UnauthorizedErrorMapper{},
			err:      fmt.Errorf("ctx: %w", domain.ErrUnauthorized),
			wantCode: codes.PermissionDenied,
		},
		{
			name:    "unauthorized mapper ignores other errors",
			mapper:  &UnauthorizedErrorMapper{},
			err:     domain.ErrMediaNotFound,
			wantNil: true,
		},
		{
			name:     "not found mapper matches ErrMediaNotFound",
			mapper:   &NotFoundErrorMapper{},
			err:      domain.ErrMediaNotFound,
			wantCode: codes.NotFound,
		},
		{
			name:     "not found mapper matches wrapped error",
			mapper:   &NotFoundErrorMapper{},
			err:      fmt.Errorf("lookup: %w", domain.ErrMediaNotFound),
			wantCode: codes.NotFound,
		},
		{
			name:    "not found mapper ignores other errors",
			mapper:  &NotFoundErrorMapper{},
			err:     domain.ErrUnauthorized,
			wantNil: true,
		},
		{
			name:     "quarantined mapper matches ErrMediaQuarantined",
			mapper:   &QuarantinedErrorMapper{},
			err:      domain.ErrMediaQuarantined,
			wantCode: codes.FailedPrecondition,
		},
		{
			name:     "quarantined mapper matches wrapped error",
			mapper:   &QuarantinedErrorMapper{},
			err:      fmt.Errorf("scan: %w", domain.ErrMediaQuarantined),
			wantCode: codes.FailedPrecondition,
		},
		{
			name:    "quarantined mapper ignores other errors",
			mapper:  &QuarantinedErrorMapper{},
			err:     domain.ErrRateLimitExceeded,
			wantNil: true,
		},
		{
			name:     "rate limit mapper matches ErrRateLimitExceeded",
			mapper:   &RateLimitErrorMapper{},
			err:      domain.ErrRateLimitExceeded,
			wantCode: codes.ResourceExhausted,
		},
		{
			name:     "rate limit mapper matches wrapped error",
			mapper:   &RateLimitErrorMapper{},
			err:      fmt.Errorf("rl: %w", domain.ErrRateLimitExceeded),
			wantCode: codes.ResourceExhausted,
		},
		{
			name:    "rate limit mapper ignores other errors",
			mapper:  &RateLimitErrorMapper{},
			err:     errors.New("boom"),
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := tt.mapper.MapError(tt.err)
			if tt.wantNil {
				if mapped != nil {
					t.Fatalf("expected nil, got %v", mapped)
				}
				return
			}
			if status.Code(mapped) != tt.wantCode {
				t.Fatalf("code = %v, want %v (err=%v)", status.Code(mapped), tt.wantCode, mapped)
			}
		})
	}
}

func TestErrorMapperRegistry(t *testing.T) {
	registry := newErrorMapperRegistry()
	if got := registry.mapError(domain.ErrMediaNotFound); got != nil {
		t.Fatalf("empty registry should not map, got %v", got)
	}

	registry.register(&NotFoundErrorMapper{})
	registry.register(&UnauthorizedErrorMapper{})

	if got := registry.mapError(domain.ErrMediaNotFound); status.Code(got) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound", status.Code(got))
	}
	if got := registry.mapError(domain.ErrUnauthorized); status.Code(got) != codes.PermissionDenied {
		t.Fatalf("code = %v, want PermissionDenied", status.Code(got))
	}
	if got := registry.mapError(errors.New("unknown")); got != nil {
		t.Fatalf("expected nil for unmapped error, got %v", got)
	}
}

type sentinelErrorMapper struct{ sentinel error }

func (m *sentinelErrorMapper) MapError(err error) error {
	if errors.Is(err, m.sentinel) {
		return status.Error(codes.Aborted, "sentinel")
	}
	return nil
}

func TestErrorMapperRegistryOrdering(t *testing.T) {
	sentinel := errors.New("sentinel")
	registry := newErrorMapperRegistry()
	registry.register(&sentinelErrorMapper{sentinel: sentinel})
	registry.register(&NotFoundErrorMapper{})

	// First registered mapper wins.
	got := registry.mapError(fmt.Errorf("wrap: %w", sentinel))
	if status.Code(got) != codes.Aborted {
		t.Fatalf("code = %v, want Aborted", status.Code(got))
	}

	// Later mappers still apply when earlier ones return nil.
	registry.register(&NotFoundErrorMapper{})
	got = registry.mapError(domain.ErrMediaNotFound)
	if status.Code(got) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound", status.Code(got))
	}
}

func TestBuildErrorMapperRegistry(t *testing.T) {
	registry := BuildErrorMapperRegistry()
	if registry == nil {
		t.Fatal("BuildErrorMapperRegistry returned nil")
	}
	if len(registry.mappers) != 4 {
		t.Fatalf("expected 4 registered mappers, got %d", len(registry.mappers))
	}
	if got := registry.mapError(domain.ErrMediaQuarantined); status.Code(got) != codes.FailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", status.Code(got))
	}
}

func TestToGRPCStatus(t *testing.T) {
	registry := BuildErrorMapperRegistry()

	tests := []struct {
		name     string
		registry *errorMapperRegistry
		err      error
		wantCode codes.Code
		wantMsg  string
	}{
		{
			name:     "media not found maps to NotFound",
			registry: registry,
			err:      domain.ErrMediaNotFound,
			wantCode: codes.NotFound,
			wantMsg:  "media not found",
		},
		{
			name:     "unauthorized maps to PermissionDenied",
			registry: registry,
			err:      domain.ErrUnauthorized,
			wantCode: codes.PermissionDenied,
			wantMsg:  "access denied",
		},
		{
			name:     "quarantined maps to FailedPrecondition",
			registry: registry,
			err:      domain.ErrMediaQuarantined,
			wantCode: codes.FailedPrecondition,
			wantMsg:  "media is quarantined",
		},
		{
			name:     "rate limit maps to ResourceExhausted",
			registry: registry,
			err:      domain.ErrRateLimitExceeded,
			wantCode: codes.ResourceExhausted,
			wantMsg:  "rate limit exceeded",
		},
		{
			name:     "validation error falls back to Internal",
			registry: registry,
			err:      fmt.Errorf("validation: %w", domain.ErrInvalidFileType),
			wantCode: codes.Internal,
			wantMsg:  "internal error",
		},
		{
			name:     "quota error falls back to Internal",
			registry: registry,
			err:      domain.ErrQuotaExceeded,
			wantCode: codes.Internal,
			wantMsg:  "internal error",
		},
		{
			name:     "context deadline falls back to Internal",
			registry: registry,
			err:      context.DeadlineExceeded,
			wantCode: codes.Internal,
			wantMsg:  "internal error",
		},
		{
			name:     "unknown error falls back to Internal",
			registry: registry,
			err:      errors.New("boom"),
			wantCode: codes.Internal,
			wantMsg:  "internal error",
		},
		{
			name:     "wrapped unknown error falls back to Internal",
			registry: registry,
			err:      fmt.Errorf("s3: %w", domain.ErrS3OperationFailed),
			wantCode: codes.Internal,
			wantMsg:  "internal error",
		},
		{
			name:     "empty registry falls back to Internal",
			registry: newErrorMapperRegistry(),
			err:      domain.ErrMediaNotFound,
			wantCode: codes.Internal,
			wantMsg:  "internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toGRPCStatus(tt.err, tt.registry)
			if status.Code(got) != tt.wantCode {
				t.Fatalf("code = %v, want %v (err=%v)", status.Code(got), tt.wantCode, got)
			}
			if msg := status.Convert(got).Message(); !strings.Contains(msg, tt.wantMsg) {
				t.Fatalf("message %q does not contain %q", msg, tt.wantMsg)
			}
		})
	}
}
