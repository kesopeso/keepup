package config

import (
	"encoding/base64"
	"testing"
)

func TestRecoveryConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name      string
		overrides map[string]string
		valid     bool
	}{
		{"SMTP auto selection", nil, true},
		{"missing key", map[string]string{"AUTH_OUTBOX_KEY": ""}, false},
		{"invalid key", map[string]string{"AUTH_OUTBOX_KEY": "not-a-key"}, false},
		{"missing origin", map[string]string{"AUTH_PUBLIC_URL": ""}, false},
		{"insecure origin", map[string]string{"AUTH_PUBLIC_URL": "http://keepup.example"}, false},
		{"origin query", map[string]string{"AUTH_PUBLIC_URL": "https://keepup.example?token=bad"}, false},
		{"missing sender", map[string]string{"SMTP_FROM": ""}, false},
		{"missing provider", map[string]string{"SMTP_HOST": ""}, false},
		{"unknown provider", map[string]string{"AUTH_MAIL_PROVIDER": "missing"}, false},
		{"fake forbidden production", map[string]string{"AUTH_MAIL_PROVIDER": "fake"}, false},
		{"plaintext forbidden", map[string]string{"SMTP_TLS_MODE": "none"}, false},
		{"partial credentials", map[string]string{"SMTP_USERNAME": "user", "SMTP_PASSWORD": ""}, false},
		{"invalid lifetime", map[string]string{"AUTH_RESET_TTL": "0s"}, false},
		{"invalid retention", map[string]string{"AUTH_RETENTION": "5m"}, false},
		{"retention covers cooldown", map[string]string{"AUTH_RESEND_COOLDOWN": "240h"}, false},
		{"invalid retry", map[string]string{"AUTH_MAIL_RETRY_DELAYS": "1m,0s"}, false},
		{"no retries", map[string]string{"AUTH_MAIL_RETRY_DELAYS": ""}, true},
		{"bad proxy", map[string]string{"AUTH_TRUSTED_PROXIES": "10.0.0.1"}, false},
		{"trusted proxy", map[string]string{"AUTH_TRUSTED_PROXIES": "10.0.0.0/24,2001:db8::/32"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for key, value := range map[string]string{
				"AUTH_ENABLED": "true", "AUTH_ALLOWED_ORIGINS": "https://keepup.example", "AUTH_PUBLIC_URL": "https://keepup.example",
				"AUTH_OUTBOX_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32)), "AUTH_MAIL_PROVIDER": "",
				"SMTP_HOST": "smtp.example", "SMTP_PORT": "587", "SMTP_FROM": "keepup@example.com", "SMTP_TLS_MODE": "starttls", "SMTP_USERNAME": "", "SMTP_PASSWORD": "",
			} {
				t.Setenv(key, value)
			}
			for key, value := range tc.overrides {
				t.Setenv(key, value)
			}
			cfg, err := loadAuth("production")
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if tc.valid && cfg.MailProvider != "smtp" {
				t.Fatal("SMTP settings did not select SMTP")
			}
		})
	}
}
