package logging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithLogContextSkipsEmptyFields(t *testing.T) {
	ctx := WithLogContext(context.Background(), LogContext{})

	assert.Equal(t, "", GetTraceID(ctx))
	assert.Equal(t, "", GetUserID(ctx))
	assert.Equal(t, "", GetMethod(ctx))
	assert.Equal(t, "", GetPath(ctx))
}

func TestWithLogContextPopulatesEveryField(t *testing.T) {
	ctx := WithLogContext(context.Background(), LogContext{
		TraceID: "tr-1",
		UserID:  "u-1",
		Method:  "GET",
		Path:    "/v1/deliveries",
	})

	assert.Equal(t, "tr-1", GetTraceID(ctx))
	assert.Equal(t, "u-1", GetUserID(ctx))
	assert.Equal(t, "GET", GetMethod(ctx))
	assert.Equal(t, "/v1/deliveries", GetPath(ctx))
}

func TestIndividualContextHelpers(t *testing.T) {
	ctx := WithTraceID(context.Background(), "t-1")
	assert.Equal(t, "t-1", GetTraceID(ctx))

	ctx = WithUserID(ctx, "u-1")
	assert.Equal(t, "u-1", GetUserID(ctx))
}

func TestGettersFallBackToRawStringKeys(t *testing.T) {
	ctx := context.WithValue(context.Background(), "x-correlation-id", "corr-1")
	ctx = context.WithValue(ctx, "x-user-id", "raw-user")

	assert.Equal(t, "corr-1", GetTraceID(ctx))
	assert.Equal(t, "raw-user", GetUserID(ctx))
}

func TestGettersOnEmptyContext(t *testing.T) {
	ctx := context.Background()

	assert.Equal(t, "", GetTraceID(ctx))
	assert.Equal(t, "", GetUserID(ctx))
	assert.Equal(t, "", GetMethod(ctx))
	assert.Equal(t, "", GetPath(ctx))
}

func TestFromContextAttachesFields(t *testing.T) {
	ctx := WithTraceID(context.Background(), "tr-9")
	ctx = WithUserID(ctx, "u-9")

	logger := FromContext(ctx)
	assert.NotNil(t, logger)

	// No context values: falls back to the default logger.
	assert.NotNil(t, FromContext(context.Background()))
}
