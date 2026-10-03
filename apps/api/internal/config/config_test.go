package config

import (
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	testCases := []struct {
		name                          string
		env                           map[string]string
		wantErr                       bool
		wantPort                      string
		wantEnv                       string
		wantTimeout                   time.Duration
		wantWebSocketAuthTimeout      time.Duration
		wantWebSocketPingInterval     time.Duration
		wantWebSocketPingTimeout      time.Duration
		wantDefaultMaxTrackingMembers int
	}{
		{
			name: "uses defaults when optional values are absent",
			env: map[string]string{
				"DATABASE_URL": "postgres://keepup:keepup@postgres:5432/keepup?sslmode=disable",
			},
			wantPort:                      defaultAppPort,
			wantEnv:                       defaultAppEnv,
			wantTimeout:                   defaultDatabaseStartupWindow,
			wantWebSocketAuthTimeout:      defaultWebSocketAuthTimeout,
			wantWebSocketPingInterval:     defaultWebSocketPingInterval,
			wantWebSocketPingTimeout:      defaultWebSocketPingTimeout,
			wantDefaultMaxTrackingMembers: defaultMaxTrackingMembers,
		},
		{
			name: "uses explicit values",
			env: map[string]string{
				"APP_ENV":                      "test",
				"APP_PORT":                     "9090",
				"DATABASE_URL":                 "postgres://keepup:keepup@postgres:5432/keepup?sslmode=disable",
				"DATABASE_STARTUP_TIMEOUT":     "45s",
				"WEBSOCKET_AUTH_TIMEOUT":       "3s",
				"WEBSOCKET_PING_INTERVAL":      "25s",
				"WEBSOCKET_PING_TIMEOUT":       "4s",
				"DEFAULT_MAX_TRACKING_MEMBERS": "14",
			},
			wantPort:                      "9090",
			wantEnv:                       "test",
			wantTimeout:                   45 * time.Second,
			wantWebSocketAuthTimeout:      3 * time.Second,
			wantWebSocketPingInterval:     25 * time.Second,
			wantWebSocketPingTimeout:      4 * time.Second,
			wantDefaultMaxTrackingMembers: 14,
		},
		{
			name:    "fails when database url is missing",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name: "fails on invalid duration",
			env: map[string]string{
				"DATABASE_URL":             "postgres://keepup:keepup@postgres:5432/keepup?sslmode=disable",
				"DATABASE_STARTUP_TIMEOUT": "not-a-duration",
			},
			wantErr: true,
		},
		{
			name: "fails on non-positive websocket auth timeout",
			env: map[string]string{
				"DATABASE_URL":           "postgres://keepup:keepup@postgres:5432/keepup?sslmode=disable",
				"WEBSOCKET_AUTH_TIMEOUT": "0s",
			},
			wantErr: true,
		},
		{
			name: "fails on non-positive websocket ping interval",
			env: map[string]string{
				"DATABASE_URL":            "postgres://keepup:keepup@postgres:5432/keepup?sslmode=disable",
				"WEBSOCKET_PING_INTERVAL": "0s",
			},
			wantErr: true,
		},
		{
			name: "fails on non-positive websocket ping timeout",
			env: map[string]string{
				"DATABASE_URL":           "postgres://keepup:keepup@postgres:5432/keepup?sslmode=disable",
				"WEBSOCKET_PING_TIMEOUT": "0s",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", "")
			t.Setenv("APP_PORT", "")
			t.Setenv("DATABASE_URL", "")
			t.Setenv("DATABASE_STARTUP_TIMEOUT", "")
			t.Setenv("WEBSOCKET_AUTH_TIMEOUT", "")
			t.Setenv("WEBSOCKET_PING_INTERVAL", "")
			t.Setenv("WEBSOCKET_PING_TIMEOUT", "")
			t.Setenv("DEFAULT_MAX_TRACKING_MEMBERS", "")

			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			cfg, err := Load()
			if tc.wantErr {
				if err == nil {
					t.Fatal("Load() error = nil, want error")
				}

				return
			}

			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			if cfg.App.Env != tc.wantEnv {
				t.Fatalf("Load() env = %q, want %q", cfg.App.Env, tc.wantEnv)
			}

			if cfg.App.Port != tc.wantPort {
				t.Fatalf("Load() port = %q, want %q", cfg.App.Port, tc.wantPort)
			}

			if cfg.Database.StartupTimeout != tc.wantTimeout {
				t.Fatalf("Load() startup timeout = %v, want %v", cfg.Database.StartupTimeout, tc.wantTimeout)
			}

			if cfg.App.WebSocketAuthTimeout != tc.wantWebSocketAuthTimeout {
				t.Fatalf("Load() websocket auth timeout = %v, want %v", cfg.App.WebSocketAuthTimeout, tc.wantWebSocketAuthTimeout)
			}

			if cfg.App.WebSocketPingInterval != tc.wantWebSocketPingInterval {
				t.Fatalf("Load() websocket ping interval = %v, want %v", cfg.App.WebSocketPingInterval, tc.wantWebSocketPingInterval)
			}

			if cfg.App.WebSocketPingTimeout != tc.wantWebSocketPingTimeout {
				t.Fatalf("Load() websocket ping timeout = %v, want %v", cfg.App.WebSocketPingTimeout, tc.wantWebSocketPingTimeout)
			}

			if cfg.Routes.DefaultMaxTrackingMembers != tc.wantDefaultMaxTrackingMembers {
				t.Fatalf("Load() default max tracking members = %d, want %d", cfg.Routes.DefaultMaxTrackingMembers, tc.wantDefaultMaxTrackingMembers)
			}
		})
	}
}

func TestGPSConfig(t *testing.T) {
	tests := []struct {
		key, value string
		invalid    bool
	}{
		{"GPS_MAX_ACCURACY_M", "50", false}, {"GPS_MAX_SPEED_MPS", "300", false},
		{"GPS_MAX_AGE", "1m", false}, {"GPS_MAX_FUTURE_SKEW", "10s", false},
		{"GPS_MAX_ACCURACY_M", "NaN", true}, {"GPS_MAX_SPEED_MPS", "+Inf", true},
		{"GPS_MAX_ACCURACY_M", "0", true}, {"GPS_MAX_SPEED_MPS", "-1", true},
		{"GPS_MAX_ACCURACY_M", "bad", true}, {"GPS_MAX_AGE", "0s", true},
		{"GPS_MAX_FUTURE_SKEW", "bad", true},
	}
	for _, tt := range tests {
		t.Run(tt.key+tt.value, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://test")
			for _, key := range []string{"GPS_MAX_ACCURACY_M", "GPS_MAX_SPEED_MPS", "GPS_MAX_AGE", "GPS_MAX_FUTURE_SKEW"} {
				t.Setenv(key, "")
			}
			t.Setenv(tt.key, tt.value)
			cfg, err := Load()
			if tt.invalid {
				if err == nil {
					t.Fatal("invalid configuration accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			policy := DefaultPositionValidation()
			switch tt.key {
			case "GPS_MAX_ACCURACY_M":
				policy.MaxAccuracyM = 50
			case "GPS_MAX_SPEED_MPS":
				policy.MaxSpeedMPS = 300
			case "GPS_MAX_AGE":
				policy.MaxAge = time.Minute
			case "GPS_MAX_FUTURE_SKEW":
				policy.MaxFutureSkew = 10 * time.Second
			}
			if cfg.Routes.PositionValidation != policy {
				t.Fatalf("got %+v want %+v", cfg.Routes.PositionValidation, policy)
			}
		})
	}
}
