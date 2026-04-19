package config

import "testing"

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
