package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration loaded from the environment.
type Config struct {
	Port           string
	DataDir        string
	MaxUploadBytes int64
	Retention      time.Duration
	BaseURL        string
}

// Load reads configuration from environment variables, applying defaults.
func Load() Config {
	return Config{
		Port:           getenv("PORT", "8080"),
		DataDir:        getenv("DATA_DIR", "./data"),
		MaxUploadBytes: getenvInt64("MAX_UPLOAD_BYTES", 100*1024*1024),
		Retention:      getenvDuration("RETENTION", 30*24*time.Hour),
		BaseURL:        os.Getenv("BASE_URL"),
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
