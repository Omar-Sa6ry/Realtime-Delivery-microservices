package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func TestNewTracerWithoutEndpoint(t *testing.T) {
	tr, err := NewTracer(TracerConfig{ServiceName: "payment-service"})
	require.NoError(t, err)
	assert.Nil(t, tr)
}

func TestNewTracerDefaultsServiceName(t *testing.T) {
	// empty ServiceName falls back to payment-service
	tr, err := NewTracer(TracerConfig{OTLPEndpoint: "localhost:4317"})
	require.NoError(t, err)
	require.NotNil(t, tr)
	assert.Equal(t, "payment-service", tr.serviceName)
	require.NoError(t, tr.Shutdown(context.Background()))
}

func TestNewTracerSuccessAndFailure(t *testing.T) {
	tr, err := NewTracer(TracerConfig{
		ServiceName:  "payment-service",
		OTLPEndpoint: "localhost:4317",
		SamplingRate: 1.0,
	})
	require.NoError(t, err)
	require.NotNil(t, tr)

	assert.NotNil(t, tr.Tracer())

	_, span := tr.StartSpan(context.Background(), "payment.create")
	span.End()
	require.NoError(t, tr.Shutdown(context.Background()))

	// invalid endpoint (control character) makes the OTLP exporter fail
	bad, err := NewTracer(TracerConfig{ServiceName: "svc", OTLPEndpoint: "\x00"})
	require.Error(t, err)
	assert.Nil(t, bad)
	assert.Contains(t, err.Error(), "failed to create OTLP exporter")
}

func TestTracerStartSpanWithAttributes(t *testing.T) {
	tr, err := NewTracer(TracerConfig{ServiceName: "payment-service", OTLPEndpoint: "localhost:4317"})
	require.NoError(t, err)
	require.NotNil(t, tr)
	defer func() { _ = tr.Shutdown(context.Background()) }()

	ctx, span := tr.StartSpanWithAttributes(context.Background(), "payment.capture",
		attribute.String("payment.id", "pay_1"))
	require.NotNil(t, ctx)
	require.NotNil(t, span)
	span.End()
}

func TestShutdownOnTracerWithoutProvider(t *testing.T) {
	// provider is nil when a Tracer is built directly (defensive branch)
	tr := &Tracer{}
	require.NoError(t, tr.Shutdown(context.Background()))
}

func TestCurrentTracerSetAndRestore(t *testing.T) {
	prev := CurrentTracer()
	defer setCurrentTracer(prev)

	setCurrentTracer(nil)
	assert.Nil(t, CurrentTracer())

	tr, err := NewTracer(TracerConfig{ServiceName: "payment-service", OTLPEndpoint: "localhost:4317"})
	require.NoError(t, err)
	require.NotNil(t, tr)
	defer func() { _ = tr.Shutdown(context.Background()) }()

	setCurrentTracer(tr)
	assert.Equal(t, tr, CurrentTracer())
}

func TestStartOperationWithoutTracer(t *testing.T) {
	prev := CurrentTracer()
	defer setCurrentTracer(prev)
	setCurrentTracer(nil)

	op := StartOperation(context.Background(), "create", "pay_1")
	require.NotNil(t, op)
	assert.NotNil(t, op.Context())
	assert.False(t, op.start.IsZero())

	// nil span guards must not panic
	op.AddAttribute("amount_minor", 100)
	op.Finish(nil)
	op.Finish(errors.New("boom"))
}

func TestStartOperationWithTracer(t *testing.T) {
	prev := CurrentTracer()
	defer setCurrentTracer(prev)

	tr, err := NewTracer(TracerConfig{ServiceName: "payment-service", OTLPEndpoint: "localhost:4317"})
	require.NoError(t, err)
	require.NotNil(t, tr)
	defer func() { _ = tr.Shutdown(context.Background()) }()
	setCurrentTracer(tr)

	op := StartOperation(context.Background(), "capture", "pay_1")
	require.NotNil(t, op)
	ctx := op.Context()
	require.NotNil(t, ctx)
	assert.NotEqual(t, context.Background(), ctx)

	op.AddAttribute("payment.amount_minor", int64(500))
	op.Finish(nil)

	op2 := StartOperation(context.Background(), "refund", "pay_2")
	op2.Finish(errors.New("failed"))
}

func TestStartPaymentOperationDirectly(t *testing.T) {
	tr, err := NewTracer(TracerConfig{ServiceName: "payment-service", OTLPEndpoint: "localhost:4317"})
	require.NoError(t, err)
	require.NotNil(t, tr)
	defer func() { _ = tr.Shutdown(context.Background()) }()

	op := StartPaymentOperation(tr, context.Background(), "authorize", "pay_9")
	require.NotNil(t, op)
	require.NotNil(t, op.Context())
	op.AddAttribute("k", "v")
	op.Finish(errors.New("x"))
	assert.Equal(t, "authorize", op.name)
}

func TestExtractTraceAndSpanIDs(t *testing.T) {
	// no span -> invalid context -> empty strings
	assert.Equal(t, "", ExtractTraceID(context.Background()))
	assert.Equal(t, "", ExtractSpanID(context.Background()))

	tr, err := NewTracer(TracerConfig{ServiceName: "payment-service", OTLPEndpoint: "localhost:4317"})
	require.NoError(t, err)
	require.NotNil(t, tr)
	defer func() { _ = tr.Shutdown(context.Background()) }()

	ctx, span := tr.StartSpan(context.Background(), "op")
	defer span.End()

	assert.Len(t, ExtractTraceID(ctx), 32)
	assert.Len(t, ExtractSpanID(ctx), 16)
}

func TestAddEventRecordErrorSetStatus(t *testing.T) {
	tr, err := NewTracer(TracerConfig{ServiceName: "payment-service", OTLPEndpoint: "localhost:4317"})
	require.NoError(t, err)
	require.NotNil(t, tr)
	defer func() { _ = tr.Shutdown(context.Background()) }()

	ctx, span := tr.StartSpan(context.Background(), "op")
	defer span.End()

	AddEvent(ctx, "provider.called", attribute.String("provider", "stripe"))
	RecordError(ctx, nil) // nil error branch
	RecordError(ctx, errors.New("declined"), attribute.String("k", "v"))
	SetStatus(ctx, 0, "ok")
	SetStatus(ctx, 500, "boom")

	// no-op span in a bare context must not panic
	AddEvent(context.Background(), "noop")
	RecordError(context.Background(), errors.New("x"))
	SetStatus(context.Background(), 0, "ok")
}

func TestPaymentAttributesFromContext(t *testing.T) {
	ctx := WithPaymentAttributes(context.Background(), "pay_1", "del_1", "usr_1", 500, "USD")
	pid, did, uid, amount, currency := GetPaymentAttributes(ctx)
	assert.Equal(t, "pay_1", pid)
	assert.Equal(t, "del_1", did)
	assert.Equal(t, "usr_1", uid)
	assert.Equal(t, int64(500), amount)
	assert.Equal(t, "USD", currency)

	pid, did, uid, amount, currency = GetPaymentAttributes(context.Background())
	assert.Equal(t, "", pid)
	assert.Equal(t, "", did)
	assert.Equal(t, "", uid)
	assert.Equal(t, int64(0), amount)
	assert.Equal(t, "", currency)
}

func TestGetTracerFromContext(t *testing.T) {
	require.NotNil(t, GetTracerFromContext(context.Background()))

	tr, err := NewTracer(TracerConfig{ServiceName: "payment-service", OTLPEndpoint: "localhost:4317"})
	require.NoError(t, err)
	require.NotNil(t, tr)
	defer func() { _ = tr.Shutdown(context.Background()) }()

	ctx, span := tr.StartSpan(context.Background(), "op")
	defer span.End()
	require.NotNil(t, GetTracerFromContext(ctx))
}

func TestSpanHelperFunctions(t *testing.T) {
	tr, err := NewTracer(TracerConfig{ServiceName: "payment-service", OTLPEndpoint: "localhost:4317"})
	require.NoError(t, err)
	require.NotNil(t, tr)
	defer func() { _ = tr.Shutdown(context.Background()) }()

	var _ trace.Tracer = tr.Tracer()
}
