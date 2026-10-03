// Package config loads runtime configuration for the KeepUp API.
package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"
)

const (
	defaultAppEnv                = "development"
	defaultAppPort               = "8080"
	defaultDatabaseStartupWindow = 20 * time.Second
	defaultWebSocketAuthTimeout  = 5 * time.Second
	defaultWebSocketPingInterval = 30 * time.Second
	defaultWebSocketPingTimeout  = 10 * time.Second
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
	WebSocketPingInterval time.Duration
	WebSocketPingTimeout  time.Duration
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
	PositionValidation        PositionValidationConfig
}

// PositionValidationConfig contains limits for accepted live GPS samples.
type PositionValidationConfig struct {
	MaxAccuracyM  float64
	MaxSpeedMPS   float64
	MaxAge        time.Duration
	MaxFutureSkew time.Duration
}

// DefaultPositionValidation returns the live GPS validation defaults.
func DefaultPositionValidation() PositionValidationConfig {
	return PositionValidationConfig{MaxAccuracyM: 100, MaxSpeedMPS: 400, MaxAge: 2 * time.Minute, MaxFutureSkew: 30 * time.Second}
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

	webSocketPingInterval, err := positiveDurationOrDefault("WEBSOCKET_PING_INTERVAL", defaultWebSocketPingInterval)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	cfg.App.WebSocketPingInterval = webSocketPingInterval

	webSocketPingTimeout, err := positiveDurationOrDefault("WEBSOCKET_PING_TIMEOUT", defaultWebSocketPingTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	cfg.App.WebSocketPingTimeout = webSocketPingTimeout

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
	policy := DefaultPositionValidation()
	for _, item := range []struct {
		key    string
		target *float64
	}{
		{"GPS_MAX_ACCURACY_M", &policy.MaxAccuracyM}, {"GPS_MAX_SPEED_MPS", &policy.MaxSpeedMPS},
	} {
		if raw := os.Getenv(item.key); raw != "" {
			value, parseErr := strconv.ParseFloat(raw, 64)
			if parseErr != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
				return Config{}, fmt.Errorf("load config: %s must be finite and greater than zero", item.key)
			}
			*item.target = value
		}
	}
	policy.MaxAge, err = positiveDurationOrDefault("GPS_MAX_AGE", policy.MaxAge)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	policy.MaxFutureSkew, err = positiveDurationOrDefault("GPS_MAX_FUTURE_SKEW", policy.MaxFutureSkew)
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	cfg.Routes.PositionValidation = policy

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
