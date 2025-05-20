package alloyinterface

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

func TestNewAlloyClient_Success(t *testing.T) {
	client, err := NewAlloyClient(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Logger)
	assert.NotNil(t, client.Tracer)
}

func TestNewAlloyClient_TracerError(t *testing.T) {
	original := initTracerFn
	defer func() { initTracerFn = original }()
	initTracerFn = func(ctx context.Context, cfg Config) (trace.Tracer, func(context.Context) error, error) {
		return nil, nil, errors.New("tracer init failed")
	}

	client, err := NewAlloyClient(context.Background())
	assert.Nil(t, client)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tracer init failed")
}

func TestNewAlloyClient_LoggerError(t *testing.T) {
	origTracer := initTracerFn
	origLogger := initLogFn
	defer func() {
		initTracerFn = origTracer
		initLogFn = origLogger
	}()

	initTracerFn = func(ctx context.Context, cfg Config) (trace.Tracer, func(context.Context) error, error) {
		return trace.NewNoopTracerProvider().Tracer("noop"), func(context.Context) error { return nil }, nil
	}
	initLogFn = func() (zerolog.Logger, error) {
		return zerolog.Logger{}, errors.New("log init failed")
	}

	client, err := NewAlloyClient(context.Background())
	assert.Nil(t, client)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "log init failed")
}

func TestAddLog_Success(t *testing.T) {
	ctx := context.WithValue(context.Background(), "request_id", "abc-123")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var payload map[string]interface{}
		json.Unmarshal(body, &payload)
		assert.Equal(t, "info message", payload["message"])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewAlloyClient(context.Background())
	client.cfg.LogEndpoint = server.URL

	resp, err := client.AddLog(ctx, zerolog.InfoLevel, "info message")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAddLog_InvalidLevel(t *testing.T) {
	client, _ := NewAlloyClient(context.Background())
	_, err := client.AddLog(context.Background(), zerolog.Level(-99), "test")
	assert.Error(t, err)
	assert.Equal(t, "invalid log level", err.Error())
}

func TestAddLog_EmptyMessage(t *testing.T) {
	client, _ := NewAlloyClient(context.Background())
	_, err := client.AddLog(context.Background(), zerolog.InfoLevel, "")
	assert.Error(t, err)
	assert.Equal(t, "log message cannot be empty", err.Error())
}

func TestAddLog_NoRequestID(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewAlloyClient(context.Background())
	client.cfg.LogEndpoint = server.URL

	resp, err := client.AddLog(ctx, zerolog.InfoLevel, "message with no request_id")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// 🚧 AddLog - HTTP failure
func TestAddLog_HttpFailure(t *testing.T) {
	client, _ := NewAlloyClient(context.Background())
	client.cfg.LogEndpoint = "http://nonexistent.invalid"

	_, err := client.AddLog(context.Background(), zerolog.InfoLevel, "fail this")
	assert.Error(t, err)
}

func TestAddLog_NonSuccessStatus(t *testing.T) {
	ctx := context.WithValue(context.Background(), "request_id", "456")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	client, _ := NewAlloyClient(context.Background())
	client.cfg.LogEndpoint = server.URL

	resp, err := client.AddLog(ctx, zerolog.InfoLevel, "test non-200")
	assert.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAddSpanWithAttr_Success(t *testing.T) {
	client, _ := NewAlloyClient(context.Background())
	err := client.AddSpanWithAttr(context.Background(), "test-span", attribute.String("foo", "bar"))
	assert.NoError(t, err)
}

func TestAddSpanWithAttr_NoTracer(t *testing.T) {
	client := &AlloyClient{}
	err := client.AddSpanWithAttr(context.Background(), "no-span", attribute.String("a", "b"))
	assert.Error(t, err)
	assert.Equal(t, "tracer not initialized", err.Error())
}

func TestAddSpan_Success(t *testing.T) {
	client, _ := NewAlloyClient(context.Background())
	err := client.AddSpan(context.Background(), "span1", "title", "message")
	assert.NoError(t, err)
}

func TestAddSpan_NoTracer(t *testing.T) {
	client := &AlloyClient{}
	err := client.AddSpan(context.Background(), "span2", "key", "val")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tracer not initialized")
}

func TestSetRateLimit(t *testing.T) {
	client, _ := NewAlloyClient(context.Background())
	client.SetRateLimit(5, 15)

	assert.Equal(t, rate.Limit(5), client.rateLimiter.Limit())
	assert.Equal(t, 15, client.rateLimiter.Burst())
}

func TestAddLog_RateLimitExceeded(t *testing.T) {
	client, _ := NewAlloyClient(context.Background())
	client.rateLimiter = rate.NewLimiter(0, 0)

	_, err := client.AddLog(context.Background(), zerolog.InfoLevel, "this should be rate-limited")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")
}

func TestShutdown(t *testing.T) {
	client, _ := NewAlloyClient(context.Background())
	err := client.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestShutdown_NoTracer(t *testing.T) {
	client := &AlloyClient{}
	err := client.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestShutdown_TracerError(t *testing.T) {
	origTracer := initTracerFn
	defer func() { initTracerFn = origTracer }()
	initTracerFn = func(ctx context.Context, cfg Config) (trace.Tracer, func(context.Context) error, error) {
		return trace.NewNoopTracerProvider().Tracer("noop"), func(context.Context) error {
			return errors.New("tracer shutdown failed")
		}, nil
	}

	client, _ := NewAlloyClient(context.Background())
	err := client.Shutdown(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tracer shutdown failed")
}

func TestStartTrace(t *testing.T) {
	client, _ := NewAlloyClient(context.Background())
	ctx, span, err := client.startTrace(context.Background(), "start-trace-test")
	assert.NoError(t, err)
	assert.NotNil(t, span)
	assert.NotNil(t, ctx)
}

func TestStartTrace_NoTracer(t *testing.T) {
	client := &AlloyClient{}
	_, _, err := client.startTrace(context.Background(), "fail-trace")
	assert.Error(t, err)
	assert.Equal(t, "tracer not initialized", err.Error())
}
