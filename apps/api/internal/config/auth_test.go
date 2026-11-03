package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func setAccountEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("AUTH_ALLOWED_ORIGINS", "https://keepup.example")
	t.Setenv("AUTH_PUBLIC_URL", "https://keepup.example")
	t.Setenv("AUTH_OUTBOX_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("EMAIL_PROVIDER", "log")
}

func TestAccountConfigurationCannotBeDisabled(t *testing.T) {
	for _, obsolete := range []string{"", "false", "true", "invalid"} {
		t.Run(obsolete, func(t *testing.T) {
			setAccountEnvironment(t)
			t.Setenv("AUTH_ENABLED", obsolete)
			if _, err := loadAuth("production"); err != nil {
				t.Fatal(err)
			}
			for _, missing := range []string{"AUTH_ALLOWED_ORIGINS", "AUTH_PUBLIC_URL", "AUTH_OUTBOX_KEY", "EMAIL_PROVIDER"} {
				t.Run(missing, func(t *testing.T) {
					t.Setenv(missing, "")
					t.Setenv("SMTP_HOST", "")
					if _, err := loadAuth("production"); err == nil {
						t.Fatalf("missing %s accepted with obsolete flag %q", missing, obsolete)
					} else if !strings.Contains(err.Error(), missing) && missing != "AUTH_OUTBOX_KEY" {
						t.Fatalf("unclear configuration error: %v", err)
					}
				})
			}
		})
	}
}

func TestAuthConfiguration(t *testing.T) {
	t.Setenv("AUTH_PUBLIC_URL", "https://keepup.example")
	t.Setenv("AUTH_OUTBOX_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	t.Setenv("SMTP_HOST", "smtp.example")
	t.Setenv("SMTP_FROM", "keepup@example.com")
	t.Setenv("EMAIL_PROVIDER", "")
	for _, tc := range []struct {
		name, env, origins, idle, attempts string
		valid                              bool
	}{
		{"local", "development", "http://localhost:3000", "2h", "4", true},
		{"production", "production", "https://keepup.example", "", "", true},
		{"missing origins", "production", "", "", "", false},
		{"wildcard", "production", "*", "", "", false},
		{"insecure production", "production", "http://keepup.example", "", "", false},
		{"url path", "production", "https://keepup.example/path", "", "", false},
		{"invalid duration", "development", "http://localhost:3000", "0s", "", false},
		{"invalid attempts", "development", "http://localhost:3000", "", "-1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
	setAccountEnvironment(t)
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
