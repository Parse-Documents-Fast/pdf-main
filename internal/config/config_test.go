package config

import (
	"testing"
)

// defaultConfig is the expected result of Load() with no environment set.
var defaultConfig = Config{
	HTTPAddr:       ":8000",
	ValidatorURL:   "http://pdf-validator:8000",
	PersistenceURL: "http://pdf-persistence:8000",
	ConverterURL:   "http://pdf-converter:8000",
	RedisQueueAddr: "redis-queue:6379",
	MaxFileSizeMB:  10,
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	got := Load()
	if got != defaultConfig {
		t.Errorf("Load() = %+v, want %+v", got, defaultConfig)
	}
}

func TestLoadFromEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("HTTP_ADDR", ":9000")
	t.Setenv("VALIDATOR_URL", "http://validator.test:9000")
	t.Setenv("PERSISTENCE_URL", "http://persist.test:9000")
	t.Setenv("CONVERTER_URL", "http://convert.test:9000")
	t.Setenv("REDIS_QUEUE_ADDR", "redis.test:6380")
	t.Setenv("MAX_FILE_SIZE_MB", "25")

	want := Config{
		HTTPAddr:       ":9000",
		ValidatorURL:   "http://validator.test:9000",
		PersistenceURL: "http://persist.test:9000",
		ConverterURL:   "http://convert.test:9000",
		RedisQueueAddr: "redis.test:6380",
		MaxFileSizeMB:  25,
	}

	if got := Load(); got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoadPartialEnv(t *testing.T) {
	clearEnv(t)
	t.Setenv("HTTP_ADDR", ":9999")

	got := Load()
	if got.HTTPAddr != ":9999" {
		t.Errorf("HTTPAddr = %q, want %q", got.HTTPAddr, ":9999")
	}
	if got.ValidatorURL != defaultConfig.ValidatorURL {
		t.Errorf("ValidatorURL = %q, want default %q", got.ValidatorURL, defaultConfig.ValidatorURL)
	}
}

func TestLoadInvalidMaxFileSizeMB(t *testing.T) {
	clearEnv(t)
	t.Setenv("MAX_FILE_SIZE_MB", "not-a-number")

	if got := Load(); got.MaxFileSizeMB != defaultConfig.MaxFileSizeMB {
		t.Errorf("MaxFileSizeMB = %d, want default %d", got.MaxFileSizeMB, defaultConfig.MaxFileSizeMB)
	}
}

// clearEnv unsets every variable read by Load so tests start from a clean slate.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HTTP_ADDR", "VALIDATOR_URL", "PERSISTENCE_URL", "CONVERTER_URL",
		"REDIS_QUEUE_ADDR", "MAX_FILE_SIZE_MB",
	} {
		t.Setenv(k, "")
	}
}
