package config

import (
	"errors"
	"os"
)

type Config struct {
	Port            string
	DatabaseURL     string
	OTLPEndpoint    string
	OTELServiceName string
}

func Load() (Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	return Config{
		Port:            getEnv("PORT", "8080"),
		DatabaseURL:     dbURL,
		OTLPEndpoint:    getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		OTELServiceName: getEnv("OTEL_SERVICE_NAME", "todo"),
	}, nil
}

func (c Config) Addr() string {
	// A bare ":port" listens on all interfaces, which containers require.
	return ":" + c.Port
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
