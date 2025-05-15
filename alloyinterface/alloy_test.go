package alloyinterface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	traceCtx, span, err := client.startTrace(ctx, "test-span")
	assert.NoError(t, err)
	assert.NotNil(t, traceCtx)
	assert.NotNil(t, span)
	assert.NotNil(t, span.SpanContext().TraceID())
}

func TestAddSpan(t *testing.T) {
	ctx := context.Background()
	client, err := NewAlloyClient(ctx)
	assert.NoError(t, err)

	err = client.AddSpanWithAttr(ctx, "test-span", attribute.String("key", "value"))
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
	// Mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate the request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Read and validate the request body
		var logRecord map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&logRecord)
		if err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		defer r.Body.Close()

		if log, ok := logRecord["log"].(map[string]interface{}); ok {
			if log["level"] != "info" {
				t.Errorf("expected level 'info', got %v", log["level"])
			}
			if log["message"] != "test message" {
				t.Errorf("expected message 'test message', got %v", log["message"])
			}
			if log["service_name"] != "TestService" {
				t.Errorf("expected service_name 'TestService', got %v", log["service_name"])
			}
		} else {
			t.Errorf("log field is missing or invalid")
		}

		// Respond with success
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Create a mock AlloyClient
	client := &AlloyClient{
		cfg: Config{
			ServiceName: "TestService",
			LogEndpoint: mockServer.URL,
		},
	}

	// Call AddLog
	ctx := context.Background()
	resp, err := client.AddLog(ctx, "info", "test message")
	if err != nil {
		t.Fatalf("AddLog failed: %v", err)
	}
	defer resp.Body.Close()

	// Validate the response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got %d", resp.StatusCode)
	}
}
