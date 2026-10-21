package config

import (
	"encoding/base64"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/maildelivery"
)

func loadRecovery(c *AuthConfig, env string) error {
	c.Recovery = auth.RecoveryConfig{PublicURL: os.Getenv("AUTH_PUBLIC_URL"), Policy: auth.DefaultRecoveryPolicy()}
	p := &c.Recovery.Policy
	durations := map[string]*time.Duration{
		"AUTH_VERIFICATION_TTL": &p.VerificationTTL, "AUTH_RESET_TTL": &p.ResetTTL,
		"AUTH_RESEND_COOLDOWN": &p.ResendCooldown, "AUTH_MAIL_WINDOW": &p.MailWindow,
		"AUTH_CONFIRM_WINDOW": &p.ConfirmWindow, "AUTH_MAIL_POLL_INTERVAL": &p.PollInterval,
		"AUTH_MAIL_TIMEOUT": &p.DeliveryTimeout, "AUTH_CLEANUP_INTERVAL": &p.CleanupInterval,
		"AUTH_RETENTION": &p.Retention, "AUTH_RECOVERY_MIN_RESPONSE": &p.MinRecoveryResponse,
	}
	for key, target := range durations {
		value, err := durationOrDefault(key, *target)
		if err != nil {
			return err
		}
		if value <= 0 {
			return fmt.Errorf("%s must be positive", key)
		}
		*target = value
	}
	for key, target := range map[string]*int{"AUTH_MAIL_PER_ACCOUNT": &p.MailPerAccount, "AUTH_MAIL_PER_SOURCE": &p.MailPerSource, "AUTH_CONFIRM_ATTEMPTS": &p.ConfirmAttempts} {
		if value := os.Getenv(key); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 10000 {
				return fmt.Errorf("%s must be between 1 and 10000", key)
			}
			*target = parsed
		}
	}
	if value, ok := os.LookupEnv("AUTH_MAIL_RETRY_DELAYS"); ok {
		p.RetryDelays = nil
		if strings.TrimSpace(value) != "" {
			for _, entry := range strings.Split(value, ",") {
				parsed, err := time.ParseDuration(strings.TrimSpace(entry))
				if err != nil || parsed <= 0 {
					return fmt.Errorf("AUTH_MAIL_RETRY_DELAYS requires positive durations")
				}
				p.RetryDelays = append(p.RetryDelays, parsed)
			}
		}
		if len(p.RetryDelays) > 20 {
			return fmt.Errorf("AUTH_MAIL_RETRY_DELAYS supports at most 20 retries")
		}
	}
	if raw := os.Getenv("AUTH_OUTBOX_KEY"); raw != "" {
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return fmt.Errorf("AUTH_OUTBOX_KEY must be base64 for 32 bytes")
		}
		c.Recovery.EncryptionKey = key
	}
	for _, value := range strings.Split(os.Getenv("AUTH_TRUSTED_PROXIES"), ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return fmt.Errorf("AUTH_TRUSTED_PROXIES requires CIDR prefixes")
		}
		c.TrustedProxies = append(c.TrustedProxies, prefix)
	}
	c.MailProvider = os.Getenv("AUTH_MAIL_PROVIDER")
	c.SMTP = maildelivery.SMTPConfig{Host: os.Getenv("SMTP_HOST"), Port: 587, Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"), From: os.Getenv("SMTP_FROM"), TLSMode: valueOrDefault("SMTP_TLS_MODE", "starttls"), Timeout: p.DeliveryTimeout}
	if raw := os.Getenv("SMTP_PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("SMTP_PORT must be an integer")
		}
		c.SMTP.Port = port
	}
	if c.MailProvider == "" && c.SMTP.Host != "" {
		c.MailProvider = "smtp"
	}
	switch c.MailProvider {
	case "smtp":
		if err := c.SMTP.Validate(); err != nil {
			return err
		}
	case "fake":
		if env != "test" {
			return fmt.Errorf("fake mail provider is only allowed in test environments")
		}
	case "":
		if c.Enabled {
			return fmt.Errorf("SMTP configuration or AUTH_MAIL_PROVIDER is required when AUTH_ENABLED is true")
		}
	default:
		return fmt.Errorf("unsupported AUTH_MAIL_PROVIDER")
	}
	if p.Retention < c.Policy.AttemptWindow || p.Retention < p.MailWindow || p.Retention < p.ConfirmWindow || p.Retention < p.ResendCooldown {
		return fmt.Errorf("AUTH_RETENTION must cover rate-limit windows")
	}
	if c.Enabled {
		if err := c.Recovery.Validate(); err != nil {
			return err
		}
		if env != "development" && !strings.HasPrefix(c.Recovery.PublicURL, "https://") {
			return fmt.Errorf("AUTH_PUBLIC_URL requires HTTPS outside development")
		}
	}
	return nil
}
