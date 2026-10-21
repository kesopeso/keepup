package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// RecoveryPolicy configures token, mail and cleanup boundaries.
type RecoveryPolicy struct {
	VerificationTTL, ResetTTL, ResendCooldown, MailWindow, ConfirmWindow           time.Duration
	MailPerAccount, MailPerSource, ConfirmAttempts                                 int
	RetryDelays                                                                    []time.Duration
	PollInterval, DeliveryTimeout, CleanupInterval, Retention, MinRecoveryResponse time.Duration
}

// DefaultRecoveryPolicy fixes the initial shared-contract defaults.
func DefaultRecoveryPolicy() RecoveryPolicy {
	return RecoveryPolicy{
		VerificationTTL: 24 * time.Hour, ResetTTL: time.Hour, ResendCooldown: time.Minute,
		MailWindow: time.Hour, ConfirmWindow: 15 * time.Minute, MailPerAccount: 5, MailPerSource: 20, ConfirmAttempts: 20,
		RetryDelays:  []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 4 * time.Hour},
		PollInterval: 5 * time.Second, DeliveryTimeout: 15 * time.Second, CleanupInterval: time.Hour, Retention: 7 * 24 * time.Hour, MinRecoveryResponse: 200 * time.Millisecond,
	}
}

// RecoveryConfig supplies a persistent secret and trusted public origin for links.
// Keep the AES-256 key outside the database; replacing it requires draining the queue.
type RecoveryConfig struct {
	PublicURL     string
	EncryptionKey []byte
	Policy        RecoveryPolicy
}

// Validate rejects incomplete settings before the account API or worker starts.
func (c RecoveryConfig) Validate() error {
	u, err := url.Parse(c.PublicURL)
	if err != nil || u.Hostname() == "" || u.User != nil || c.PublicURL != u.Scheme+"://"+u.Host || (u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("AUTH_PUBLIC_URL must be a public origin")
	}
	if len(c.EncryptionKey) != 32 {
		return fmt.Errorf("AUTH_OUTBOX_KEY must encode exactly 32 bytes")
	}
	p := c.Policy
	for _, d := range []time.Duration{p.VerificationTTL, p.ResetTTL, p.ResendCooldown, p.MailWindow, p.ConfirmWindow, p.PollInterval, p.DeliveryTimeout, p.CleanupInterval, p.Retention, p.MinRecoveryResponse} {
		if d <= 0 {
			return fmt.Errorf("recovery durations must be positive")
		}
	}
	if p.MailPerAccount < 1 || p.MailPerSource < 1 || p.ConfirmAttempts < 1 || len(p.RetryDelays) > 20 {
		return fmt.Errorf("invalid recovery limits")
	}
	for _, d := range p.RetryDelays {
		if d <= 0 {
			return fmt.Errorf("mail retry delays must be positive")
		}
	}
	if p.Retention < p.MailWindow || p.Retention < p.ConfirmWindow || p.Retention < p.ResendCooldown {
		return fmt.Errorf("retention must cover rate-limit windows")
	}
	return nil
}

type recoveryRuntime struct {
	config RecoveryConfig
	cipher cipher.AEAD
}

func newRecovery(c RecoveryConfig) (*recoveryRuntime, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(c.EncryptionKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &recoveryRuntime{config: c, cipher: aead}, nil
}

func (r *recoveryRuntime) encrypt(id, link string) []byte {
	nonce := make([]byte, r.cipher.NonceSize())
	_, _ = rand.Read(nonce)
	return r.cipher.Seal(nonce, nonce, []byte(link), []byte(id))
}

func (r *recoveryRuntime) decrypt(id string, data []byte) (string, error) {
	n := r.cipher.NonceSize()
	if len(data) < n {
		return "", fmt.Errorf("invalid encrypted mail")
	}
	text, err := r.cipher.Open(nil, data[:n], data[n:], []byte(id))
	if err != nil {
		return "", fmt.Errorf("invalid encrypted mail")
	}
	return string(text), nil
}

func (s *Service) mailSource(ctx context.Context, source string) error {
	p := s.recovery.config.Policy
	return s.repo.allowAttempt(ctx, "mail:source:"+source, p.MailWindow, p.MailPerSource)
}

// ResendVerification replaces the old link for an authenticated pending user.
func (s *Service) ResendVerification(ctx context.Context, sessionToken, source string) error {
	if _, err := s.Authenticate(ctx, sessionToken); err != nil {
		return err
	}
	if err := s.mailSource(ctx, source); err != nil {
		return err
	}
	return s.repo.resendVerification(ctx, tokenHash(sessionToken), s.recovery)
}

// ConfirmVerification activates once and never issues a session to the link opener.
func (s *Service) ConfirmVerification(ctx context.Context, token, source string) error {
	if err := s.confirmAttempt(ctx, source); err != nil {
		return err
	}
	return s.repo.consumeToken(ctx, token, "verification", "", s.recovery)
}

func (s *Service) confirmAttempt(ctx context.Context, source string) error {
	p := s.recovery.config.Policy
	return s.repo.allowAttempt(ctx, "confirm:source:"+source, p.ConfirmWindow, p.ConfirmAttempts)
}

// ForgotPassword intentionally hides absent users and account-specific cooldowns.
func (s *Service) ForgotPassword(ctx context.Context, identifier, source string) error {
	started := time.Now()
	defer func() {
		timer := time.NewTimer(time.Until(started.Add(s.recovery.config.Policy.MinRecoveryResponse)))
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
		}
	}()
	identifier = NormalizeIdentifier(identifier)
	if identifier == "" || len(identifier) > 254 {
		return ErrInvalidInput
	}
	if err := s.mailSource(ctx, source); err != nil {
		return err
	}
	err := s.repo.forgotPassword(ctx, identifier, s.recovery)
	if errors.Is(err, ErrRateLimited) {
		return nil
	}
	return err
}

// ResetPassword validates before atomically consuming a reset token and revoking all sessions.
func (s *Service) ResetPassword(ctx context.Context, token, password, source string) error {
	if err := s.confirmAttempt(ctx, source); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	if len(token) != 43 {
		return ErrInvalidToken
	}
	if err := s.acquire(ctx); err != nil {
		return err
	}
	defer func() { <-s.work }()
	return s.repo.consumeToken(ctx, token, "reset", hashPassword(password), s.recovery)
}

// ChangePassword checks current authority again inside the update transaction.
func (s *Service) ChangePassword(ctx context.Context, sessionToken, currentPassword, password, source string) (Result, error) {
	if err := ValidatePassword(password); err != nil {
		return Result{}, err
	}
	if !utf8.ValidString(currentPassword) || utf8.RuneCountInString(currentPassword) > 128 {
		return Result{}, ErrCredentials
	}
	if err := s.repo.allowAttempt(ctx, "password-change:source:"+source, s.policy.AttemptWindow, s.policy.LoginAttempts); err != nil {
		return Result{}, err
	}
	session, err := s.Authenticate(ctx, sessionToken)
	if err != nil {
		return Result{}, err
	}
	if err = RequireActive(session); err != nil {
		return Result{}, err
	}
	if err = s.repo.allowAttempt(ctx, "password-change:user:"+session.User.ID, s.policy.AttemptWindow, s.policy.LoginAttempts); err != nil {
		return Result{}, err
	}
	if err = s.acquire(ctx); err != nil {
		return Result{}, err
	}
	defer func() { <-s.work }()
	_, oldHash, err := s.repo.password(ctx, strings.ToLower(session.User.Username))
	if err != nil {
		return Result{}, err
	}
	if !matchesPassword(currentPassword, oldHash) {
		return Result{}, ErrCredentials
	}
	token := newToken()
	changed, err := s.repo.changePassword(ctx, tokenHash(sessionToken), oldHash, hashPassword(password), tokenHash(token), s.policy.SessionIdle)
	if err != nil {
		return Result{}, err
	}
	return Result{Session: changed, Token: token}, nil
}
