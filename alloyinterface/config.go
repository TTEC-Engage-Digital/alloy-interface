package alloyinterface

import (
	"os"
)

type Config struct {
	Endpoint    string
	ServiceName string
	TracerName  string
}

func LoadConfig() Config {
	return Config{
		Endpoint:    getEnv("ALLOY_ENDPOINT", "http://localhost:4318"),
		ServiceName: getEnv("ALLOY_SERVICE_NAME", "addi"),
		TracerName:  getEnv("ALLOY_TRACER_NAME", "addi-tracer"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
