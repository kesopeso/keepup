package config

import (
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/maildelivery"
)

// AuthConfig gates the staged account API; route integration is delivered separately.
type AuthConfig struct {
	Enabled         bool
	Origins         []string
	Policy          auth.Policy
	Recovery        auth.RecoveryConfig
	MailProvider    string
	SMTP            maildelivery.SMTPConfig
	TrustedProxies  []netip.Prefix
	Google          auth.GoogleConfig
	SocketTicketTTL time.Duration
}

func loadAuth(env string) (AuthConfig, error) {
	cfg := AuthConfig{Policy: auth.DefaultPolicy()}
	if value := os.Getenv("AUTH_ENABLED"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			return cfg, fmt.Errorf("AUTH_ENABLED must be a boolean")
		}
		cfg.Enabled = enabled
	}
	var err error
	if cfg.Policy.SessionIdle, err = durationOrDefault("AUTH_SESSION_IDLE", cfg.Policy.SessionIdle); err != nil {
		return cfg, err
	}
	if cfg.Policy.AttemptWindow, err = durationOrDefault("AUTH_ATTEMPT_WINDOW", cfg.Policy.AttemptWindow); err != nil {
		return cfg, err
	}
	if cfg.Policy.SessionIdle <= 0 || cfg.Policy.AttemptWindow <= 0 {
		return cfg, fmt.Errorf("AUTH_SESSION_IDLE and AUTH_ATTEMPT_WINDOW must be positive")
	}
	for key, dst := range map[string]*int{"AUTH_LOGIN_ATTEMPTS": &cfg.Policy.LoginAttempts, "AUTH_SIGNUP_ATTEMPTS": &cfg.Policy.SignupAttempts} {
		if value := os.Getenv(key); value != "" {
			n, parseErr := strconv.Atoi(value)
			if parseErr != nil || n < 1 || n > 10000 {
				return cfg, fmt.Errorf("%s must be between 1 and 10000", key)
			}
			*dst = n
		}
	}
	for _, raw := range strings.Split(os.Getenv("AUTH_ALLOWED_ORIGINS"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, parseErr := url.Parse(raw)
		if parseErr != nil {
			return cfg, fmt.Errorf("invalid AUTH_ALLOWED_ORIGINS URL")
		}
		secureScheme := u.Scheme == "https" || env == "development" && u.Scheme == "http"
		if !secureScheme || u.Hostname() == "" || u.User != nil || raw != u.Scheme+"://"+u.Host || strings.Contains(raw, "*") {
			return cfg, fmt.Errorf("AUTH_ALLOWED_ORIGINS requires exact origins, HTTPS outside development")
		}
		cfg.Origins = append(cfg.Origins, raw)
	}
	if cfg.Enabled && len(cfg.Origins) == 0 {
		return cfg, fmt.Errorf("AUTH_ALLOWED_ORIGINS is required when AUTH_ENABLED is true")
	}
	if err := loadRecovery(&cfg, env); err != nil {
		return cfg, err
	}
	if err := loadGoogle(&cfg); err != nil {
		return cfg, err
	}
	if cfg.SocketTicketTTL, err = durationOrDefault("AUTH_SOCKET_TICKET_TTL", 30*time.Second); err != nil {
		return cfg, err
	}
	if cfg.SocketTicketTTL <= 0 {
		return cfg, fmt.Errorf("AUTH_SOCKET_TICKET_TTL must be positive")
	}
	return cfg, nil
}
