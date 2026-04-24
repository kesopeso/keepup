// Package config loads runtime configuration for the KeepUp API.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultAppEnv                = "development"
	defaultAppPort               = "8080"
	defaultDatabaseStartupWindow = 20 * time.Second
	defaultMaxTrackingMembers    = 10
)

// Config contains the full API runtime configuration.
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Routes   RouteConfig
}

// AppConfig contains HTTP server settings.
type AppConfig struct {
	Env  string
	Port string
}

// DatabaseConfig contains PostgreSQL connection settings.
type DatabaseConfig struct {
	URL            string
	StartupTimeout time.Duration
}

// RouteConfig contains route lifecycle defaults.
type RouteConfig struct {
	DefaultMaxTrackingMembers int
}

// Load reads the KeepUp API configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		App: AppConfig{
			Env:  valueOrDefault("APP_ENV", defaultAppEnv),
			Port: valueOrDefault("APP_PORT", defaultAppPort),
		},
		Database: DatabaseConfig{
			URL: valueOrDefault("DATABASE_URL", ""),
		},
		Routes: RouteConfig{
			DefaultMaxTrackingMembers: defaultMaxTrackingMembers,
		},
	}

	if cfg.Database.URL == "" {
		return Config{}, fmt.Errorf("load config: DATABASE_URL is required")
	}

	startupTimeout, err := durationOrDefault("DATABASE_STARTUP_TIMEOUT", defaultDatabaseStartupWindow)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	cfg.Database.StartupTimeout = startupTimeout

	maxTrackingMembers, err := intOrDefault("DEFAULT_MAX_TRACKING_MEMBERS", defaultMaxTrackingMembers)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	if maxTrackingMembers <= 0 {
		return Config{}, fmt.Errorf("load config: DEFAULT_MAX_TRACKING_MEMBERS must be greater than zero")
	}

	cfg.Routes.DefaultMaxTrackingMembers = maxTrackingMembers

	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func durationOrDefault(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return parsed, nil
}

func intOrDefault(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return parsed, nil
}
