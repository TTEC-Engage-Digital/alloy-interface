package alloyinterface

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestNewAlloyClient(t *testing.T) {
	ctx := context.Background()
	client, err := NewAlloyClient(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Tracer)
}

func TestStartTrace(t *testing.T) {
	ctx := context.Background()
	client, err := NewAlloyClient(ctx)
	assert.NoError(t, err)

	traceCtx, span, err := client.StartTrace(ctx, "test-span")
	assert.NoError(t, err)
	assert.NotNil(t, traceCtx)
	assert.NotNil(t, span)
	assert.NotNil(t, span.SpanContext().TraceID())
}

func TestAddSpan(t *testing.T) {
	ctx := context.Background()
	client, err := NewAlloyClient(ctx)
	assert.NoError(t, err)

	err = client.AddSpan(ctx, "test-span", attribute.String("key", "value"))
	assert.NoError(t, err)
}

func TestShutdown(t *testing.T) {
	ctx := context.Background()
	client, err := NewAlloyClient(ctx)
	assert.NoError(t, err)

	err = client.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestAddLog(t *testing.T) {
	ctx := context.Background()
	client, err := NewAlloyClient(ctx)
	assert.NoError(t, err)

	err = client.AddLog(ctx, slog.LevelDebug, "This is a log message")
	assert.NoError(t, err)
}
