package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const DefaultLogLevel = "info"

// ParseLogLevel maps config/env strings to slog levels.
func ParseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", DefaultLogLevel:
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q (want debug, info, warn, error)", raw)
	}
}

// BootstrapLogLevel resolves process log level from MAVEN_LOG_LEVEL or config.
func BootstrapLogLevel() (slog.Level, error) {
	if v := strings.TrimSpace(os.Getenv("MAVEN_LOG_LEVEL")); v != "" {
		return ParseLogLevel(v)
	}
	cfg, err := LoadConfig()
	if err != nil {
		return slog.LevelInfo, nil
	}
	return ParseLogLevel(cfg.Logging.Level)
}
