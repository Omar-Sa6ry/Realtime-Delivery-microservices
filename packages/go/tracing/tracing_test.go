package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func installSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
	})
	return recorder
}

func findSpan(recorder *tracetest.SpanRecorder, name string) (sdktrace.ReadOnlySpan, bool) {
	for _, s := range recorder.Ended() {
		if s.Name() == name {
			return s, true
		}
	}
	return nil, false
}

func TestUnaryServerTracingInterceptorRecordsSuccessfulCall(t *testing.T) {
	recorder := installSpanRecorder(t)
	interceptor := UnaryServerTracingInterceptor()

	var childCtxValid bool
	_, err := interceptor(context.Background(), struct{}{},
		&grpc.UnaryServerInfo{FullMethod: "/driver.DriverService/ReserveDriver"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			childCtxValid = trace.SpanContextFromContext(ctx).IsValid()
			return "ok", nil
		})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !childCtxValid {
		t.Fatal("handler context must carry a valid span context")
	}

	span, ok := findSpan(recorder, "/driver.DriverService/ReserveDriver")
	if !ok {
		t.Fatalf("span not recorded, got %d spans", len(recorder.Ended()))
	}
	if span.SpanContext().SpanID() == (trace.SpanID{}) {
		t.Fatal("span id must not be empty")
	}

	attrs := map[attribute.Key]interface{}{}
	for _, a := range span.Attributes() {
		attrs[a.Key] = a.Value.AsInterface()
	}
	if attrs[attribute.Key("rpc.system")] != "grpc" {
		t.Fatalf("rpc.system = %v, want grpc", attrs["rpc.system"])
	}
	if attrs[attribute.Key("rpc.grpc.status_code")] != int64(0) {
		t.Fatalf("rpc.grpc.status_code = %v, want 0", attrs["rpc.grpc.status_code"])
	}
	if span.Status().Code != otelcodes.Ok {
		t.Fatalf("unexpected status %+v", span.Status())
	}
}

func TestUnaryServerTracingInterceptorRecordsError(t *testing.T) {
	recorder := installSpanRecorder(t)
	interceptor := UnaryServerTracingInterceptor()

	_, err := interceptor(context.Background(), struct{}{},
		&grpc.UnaryServerInfo{FullMethod: "/driver.DriverService/FindAvailableDrivers"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, status.Error(codes.NotFound, "driver not found")
		})
	if err == nil {
		t.Fatal("expected handler error")
	}

	span, ok := findSpan(recorder, "/driver.DriverService/FindAvailableDrivers")
	if !ok {
		t.Fatal("span not recorded")
	}
	if span.Status().Code != otelcodes.Error {
		t.Fatalf("status = %+v, want error", span.Status())
	}

	var hasGRPCCode bool
	for _, a := range span.Attributes() {
		if a.Key == attribute.Key("rpc.grpc.status_code") && a.Value.AsInt64() == int64(codes.NotFound) {
			hasGRPCCode = true
		}
	}
	if !hasGRPCCode {
		t.Fatal("missing rpc.grpc.status_code attribute")
	}
	if len(span.Events()) == 0 {
		t.Fatal("expected an error event on the span")
	}
}

func TestHTTPTracingMiddlewareRecordsStatus(t *testing.T) {
	recorder := installSpanRecorder(t)

	handler := HTTPTracingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	span, ok := findSpan(recorder, "POST /graphql")
	if !ok {
		t.Fatal("span not recorded")
	}

	var statusCode interface{}
	for _, a := range span.Attributes() {
		if a.Key == attribute.Key("http.status_code") {
			statusCode = a.Value.AsInt64()
		}
	}
	if statusCode != int64(http.StatusInternalServerError) {
		t.Fatalf("http.status_code = %v, want 500", statusCode)
	}
	if span.Status().Code != otelcodes.Error {
		t.Fatal("5xx responses must mark the span as error")
	}
}

func TestHTTPTracingMiddlewarePassesSpanContextDownstream(t *testing.T) {
	installSpanRecorder(t)

	var valid bool
	handler := HTTPTracingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		valid = trace.SpanContextFromContext(r.Context()).IsValid()
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	if !valid {
		t.Fatal("downstream request context must carry a valid span context")
	}
}

func TestStartSpanCreatesInternalSpan(t *testing.T) {
	recorder := installSpanRecorder(t)

	_, span := StartSpan(context.Background(), "dispatch.reserve",
		attribute.String("driver.id", "d-1"))
	span.End()

	found, ok := findSpan(recorder, "dispatch.reserve")
	if !ok {
		t.Fatal("span not recorded")
	}
	if found.Parent().SpanID() != (trace.SpanID{}) {
		t.Fatal("internal span started without an explicit parent must be a root span")
	}
}
