package alloyconfig

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("ALLOY_ENDPOINT", "test-endpoint")
	os.Setenv("ALLOY_SERVICE_NAME", "test-service")
	os.Setenv("ALLOY_TRACER_NAME", "test-tracer")

	// Load the configuration
	cfg := LoadConfig()

	// Assert the values are correctly loaded from environment variables
	assert.Equal(t, "test-endpoint", cfg.TraceEndpoint)
	assert.Equal(t, "test-service", cfg.ServiceName)
	assert.Equal(t, "test-tracer", cfg.TracerName)

	// Clean up environment variables
	os.Unsetenv("ALLOY_ENDPOINT")
	os.Unsetenv("ALLOY_SERVICE_NAME")
	os.Unsetenv("ALLOY_TRACER_NAME")
}

func TestLoadConfigWithFallback(t *testing.T) {
	// Ensure environment variables are not set
	os.Unsetenv("ALLOY_ENDPOINT")
	os.Unsetenv("ALLOY_SERVICE_NAME")
	os.Unsetenv("ALLOY_TRACER_NAME")

	// Load the configuration
	cfg := LoadConfig()

	// Assert the fallback values are used
	assert.Equal(t, "localhost:4318", cfg.TraceEndpoint)
	assert.Equal(t, "addi", cfg.ServiceName)
	assert.Equal(t, "addi-tracer", cfg.TracerName)
}

func TestGetEnv(t *testing.T) {
	// Test with environment variable set
	os.Setenv("TEST_KEY", "test-value")
	value := getEnv("TEST_KEY", "fallback-value")
	assert.Equal(t, "test-value", value)

	// Test with environment variable not set
	os.Unsetenv("TEST_KEY")
	value = getEnv("TEST_KEY", "fallback-value")
	assert.Equal(t, "fallback-value", value)
}
