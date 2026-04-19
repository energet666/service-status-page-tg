package config

import (
	"testing"
	"time"
)

func TestLoadParsesChecksFile(t *testing.T) {
	t.Setenv("CHECKS_FILE", "config/checks.local.json")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ChecksFile != "config/checks.local.json" {
		t.Fatalf("ChecksFile = %q, want %q", cfg.ChecksFile, "config/checks.local.json")
	}
}

func TestLoadParsesChecksInterval(t *testing.T) {
	t.Setenv("CHECKS_INTERVAL", "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.CheckInterval != 30*time.Second {
		t.Fatalf("CheckInterval = %s, want 30s", cfg.CheckInterval)
	}
}

func TestLoadRejectsInvalidChecksInterval(t *testing.T) {
	t.Setenv("CHECKS_INTERVAL", "soon")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error is nil, want invalid interval error")
	}
}
