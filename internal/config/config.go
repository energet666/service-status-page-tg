package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr      string
	BotToken      string
	AdminIDs      map[int64]struct{}
	AdminIDList   []int64
	PublicBaseURL string
	DataFile      string
	ChecksFile    string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:      value("HTTP_ADDR", ":8080"),
		BotToken:      os.Getenv("BOT_TOKEN"),
		PublicBaseURL: value("PUBLIC_BASE_URL", "http://localhost:8080"),
		DataFile:      value("DATA_FILE", "data/state.json"),
		ChecksFile:    value("CHECKS_FILE", "checks.json"),
		AdminIDs:      map[int64]struct{}{},
	}

	rawAdminIDs := strings.TrimSpace(os.Getenv("ADMIN_IDS"))
	if rawAdminIDs == "" {
		return cfg, nil
	}

	for _, part := range strings.Split(rawAdminIDs, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("parse ADMIN_IDS %q: %w", part, err)
		}
		cfg.AdminIDs[id] = struct{}{}
		cfg.AdminIDList = append(cfg.AdminIDList, id)
	}

	return cfg, nil
}

func value(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
