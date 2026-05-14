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
	defaultWebSocketAuthTimeout  = 5 * time.Second
	defaultTrackingStaleAfter    = 20 * time.Second
	defaultTrackingOfflineAfter  = 5 * time.Minute
	defaultSpectatorOfflineAfter = 20 * time.Second
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
	Env                   string
	Port                  string
	WebSocketAuthTimeout  time.Duration
	TrackingStaleAfter    time.Duration
	TrackingOfflineAfter  time.Duration
	SpectatorOfflineAfter time.Duration
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

	webSocketAuthTimeout, err := durationOrDefault("WEBSOCKET_AUTH_TIMEOUT", defaultWebSocketAuthTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	if webSocketAuthTimeout <= 0 {
		return Config{}, fmt.Errorf("load config: WEBSOCKET_AUTH_TIMEOUT must be greater than zero")
	}

	cfg.App.WebSocketAuthTimeout = webSocketAuthTimeout

	trackingStaleAfter, err := positiveDurationOrDefault("ROUTES_TRACKING_STALE_AFTER", defaultTrackingStaleAfter)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	cfg.App.TrackingStaleAfter = trackingStaleAfter

	trackingOfflineAfter, err := positiveDurationOrDefault("ROUTES_TRACKING_OFFLINE_AFTER", defaultTrackingOfflineAfter)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	cfg.App.TrackingOfflineAfter = trackingOfflineAfter

	spectatorOfflineAfter, err := positiveDurationOrDefault("ROUTES_SPECTATOR_OFFLINE_AFTER", defaultSpectatorOfflineAfter)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	cfg.App.SpectatorOfflineAfter = spectatorOfflineAfter

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

func positiveDurationOrDefault(key string, fallback time.Duration) (time.Duration, error) {
	duration, err := durationOrDefault(key, fallback)
	if err != nil {
		return 0, err
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", key)
	}

	return duration, nil
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
