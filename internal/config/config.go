// Package config loads the service configuration from environment variables.
//
// The values map 1:1 to the environment variables documented in docs/spec.md
// (section "Configuración"). Every variable has a sensible default so the
// service can boot without any env set; in production these defaults are the
// addresses of the downstream services inside the shared Docker network.
package config

import (
	"log/slog"
	"os"
	"strconv"
)

// Config is the immutable service configuration loaded once at startup.
// Fields are exported (read-only by convention: no setters) so the wiring in
// cmd/pdf-main can build the adapters from them.
type Config struct {
	// HTTPAddr is the internal listen address (behind Traefik, not exposed).
	HTTPAddr string

	// ValidatorURL is the base URL of pdf-validator.
	ValidatorURL string

	// PersistenceURL is the base URL of pdf-persistence.
	PersistenceURL string

	// ConverterURL is the base URL of pdf-converter (download-only).
	ConverterURL string

	// RedisQueueAddr is the address of the Redis instance used for queues.
	RedisQueueAddr string

	// MaxFileSizeMB is the upload size limit inherited from the monolith.
	MaxFileSizeMB int
}

// Load reads the configuration from the environment, applying the defaults
// from the spec when a variable is unset or invalid. It never returns an
// error: an invalid value logs a clear warning and falls back to the default.
// This keeps the startup path simple (no config error to surface in main.go)
// while still making misconfiguration visible in the logs.
func Load() Config {
	return Config{
		HTTPAddr:       getEnv("HTTP_ADDR", ":8000"),
		ValidatorURL:   getEnv("VALIDATOR_URL", "http://pdf-validator:8000"),
		PersistenceURL: getEnv("PERSISTENCE_URL", "http://pdf-persistence:8000"),
		ConverterURL:   getEnv("CONVERTER_URL", "http://pdf-converter:8000"),
		RedisQueueAddr: getEnv("REDIS_QUEUE_ADDR", "redis-queue:6379"),
		MaxFileSizeMB:  getEnvInt("MAX_FILE_SIZE_MB", 10),
	}
}

// getEnv returns the value of key, or fallback when key is unset or empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt parses key as an int, returning fallback when it is unset, empty,
// or not a valid integer. In the last case it logs a warning so the operator
// knows a default was substituted for a bad value.
func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("invalid config value, using default",
			"var", key,
			"value", v,
			"default", fallback,
			"reason", err.Error(),
		)
		return fallback
	}
	return n
}
