package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	for _, k := range []string{"PORT", "DATA_DIR", "MAX_UPLOAD_BYTES", "RETENTION", "BASE_URL"} {
		t.Setenv(k, "")
	}
	cfg := Load()
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.DataDir != "./data" {
		t.Fatalf("DataDir = %q, want ./data", cfg.DataDir)
	}
	if cfg.MaxUploadBytes != 100*1024*1024 {
		t.Fatalf("MaxUploadBytes = %d, want %d", cfg.MaxUploadBytes, 100*1024*1024)
	}
	if cfg.Retention != 30*24*time.Hour {
		t.Fatalf("Retention = %v, want %v", cfg.Retention, 30*24*time.Hour)
	}
	if cfg.BaseURL != "" {
		t.Fatalf("BaseURL = %q, want empty", cfg.BaseURL)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATA_DIR", "/tmp/share")
	t.Setenv("MAX_UPLOAD_BYTES", "1234")
	t.Setenv("RETENTION", "48h")
	t.Setenv("BASE_URL", "https://share.example.com")
	cfg := Load()
	if cfg.Port != "9090" || cfg.DataDir != "/tmp/share" || cfg.MaxUploadBytes != 1234 ||
		cfg.Retention != 48*time.Hour || cfg.BaseURL != "https://share.example.com" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}
