package alloyinterface

import (
	"os"
)

type Config struct {
	TraceEndpoint string
	LogEndpoint   string
	CertFilePath  string
	ServiceName   string
	TracerName    string
}

func LoadConfig() Config {
	return Config{
		TraceEndpoint: getEnv("ALLOY_ENDPOINT", "localhost:4318"),
		LogEndpoint:   getEnv("ALLOY_LOG_ENDPOINT", "http://localhost:9999"),
		CertFilePath:  getEnv("ALLOY_CERTFILE_PATH", "/etc/config/grafana-alloy.crt"),
		ServiceName:   getEnv("ALLOY_SERVICE_NAME", "addi"),
		TracerName:    getEnv("ALLOY_TRACER_NAME", "addi-tracer"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
