package config

import (
	"encoding/base64"
	"testing"
)

func TestAuthConfiguration(t *testing.T) {
	t.Setenv("AUTH_PUBLIC_URL", "https://keepup.example")
	t.Setenv("AUTH_OUTBOX_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("SMTP_HOST", "smtp.example")
	t.Setenv("SMTP_FROM", "keepup@example.com")
	t.Setenv("AUTH_MAIL_PROVIDER", "")
	for _, tc := range []struct {
		name, env, enabled, origins, idle, attempts string
		valid                                       bool
	}{
		{"disabled", "development", "false", "", "", "", true},
		{"local", "development", "true", "http://localhost:3000", "2h", "4", true},
		{"production", "production", "true", "https://keepup.example", "", "", true},
		{"missing origins", "production", "true", "", "", "", false},
		{"wildcard", "production", "true", "*", "", "", false},
		{"insecure production", "production", "true", "http://keepup.example", "", "", false},
		{"url path", "production", "true", "https://keepup.example/path", "", "", false},
		{"invalid bool", "development", "maybe", "", "", "", false},
		{"invalid duration", "development", "false", "", "0s", "", false},
		{"invalid attempts", "development", "false", "", "", "-1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AUTH_ENABLED", tc.enabled)
			t.Setenv("AUTH_ALLOWED_ORIGINS", tc.origins)
			t.Setenv("AUTH_SESSION_IDLE", tc.idle)
			t.Setenv("AUTH_LOGIN_ATTEMPTS", tc.attempts)
			t.Setenv("AUTH_ATTEMPT_WINDOW", "")
			t.Setenv("AUTH_SIGNUP_ATTEMPTS", "")
			cfg, err := loadAuth(tc.env)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if tc.valid && cfg.Policy.SessionIdle <= 0 {
				t.Fatal("invalid session lifetime")
			}
		})
	}
}

func TestSocketTicketConfiguration(t *testing.T) {
	t.Setenv("AUTH_ENABLED", "false")
	for _, tc := range []struct {
		value string
		valid bool
	}{{"", true}, {"10s", true}, {"0s", false}, {"-1s", false}, {"invalid", false}} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("AUTH_SOCKET_TICKET_TTL", tc.value)
			cfg, err := loadAuth("development")
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
			if tc.valid && cfg.SocketTicketTTL <= 0 {
				t.Fatal("invalid ticket lifetime")
			}
		})
	}
}
