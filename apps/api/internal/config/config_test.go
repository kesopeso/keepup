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
			wantDefaultMaxTrackingMembers: defaultMaxTrackingMembers,
		},
		{
			name: "uses explicit values",
			env: map[string]string{
				"APP_ENV":                      "test",
				"APP_PORT":                     "9090",
				"DATABASE_URL":                 "postgres://keepup:keepup@postgres:5432/keepup?sslmode=disable",
				"DATABASE_STARTUP_TIMEOUT":     "45s",
				"DEFAULT_MAX_TRACKING_MEMBERS": "14",
			},
			wantPort:                      "9090",
			wantEnv:                       "test",
			wantTimeout:                   45 * time.Second,
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
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", "")
			t.Setenv("APP_PORT", "")
			t.Setenv("DATABASE_URL", "")
			t.Setenv("DATABASE_STARTUP_TIMEOUT", "")
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

			if cfg.Routes.DefaultMaxTrackingMembers != tc.wantDefaultMaxTrackingMembers {
				t.Fatalf("Load() default max tracking members = %d, want %d", cfg.Routes.DefaultMaxTrackingMembers, tc.wantDefaultMaxTrackingMembers)
			}
		})
	}
}
