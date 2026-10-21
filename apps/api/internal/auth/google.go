package auth

import (
	"context"
	"crypto/subtle"
	"time"
)

func (s *Service) googleAudience(transport string) ([]string, error) {
	if s.google == nil {
		return nil, ErrGoogleDisabled
	}
	var ids []string
	switch transport {
	case "web":
		ids = s.google.config.WebClientIDs
	case "native":
		ids = s.google.config.NativeClientIDs
	default:
		return nil, ErrInvalidFlow
	}
	if len(ids) == 0 {
		return nil, ErrGoogleDisabled
	}
	return ids, nil
}

func (s *Service) googleAttempt(ctx context.Context, source string) error {
	return s.repo.allowAttempt(ctx, "google:source:"+source, s.policy.AttemptWindow, s.policy.LoginAttempts)
}

// StartGoogle creates a restricted flow, never an account session.
func (s *Service) StartGoogle(ctx context.Context, transport, source string) (GoogleStart, error) {
	audiences, err := s.googleAudience(transport)
	if err != nil {
		return GoogleStart{}, err
	}
	if err = s.googleAttempt(ctx, source); err != nil {
		return GoogleStart{}, err
	}
	token, nonce := newToken(), newToken()
	result := GoogleStart{Token: token, Nonce: nonce}
	err = s.repo.pool.QueryRow(ctx, `INSERT INTO auth_google_flows (token_hash,nonce_hash,transport,audiences,expires_at)
        VALUES ($1,$2,$3,$4,clock_timestamp()+$5*interval '1 second') RETURNING expires_at`, tokenHash(token), tokenHash(nonce), transport, audiences, s.google.config.FlowTTL.Seconds()).Scan(&result.ExpiresAt)
	return result, err
}

// GoogleContext polls a flow without creating sessions or refreshing its deadline.
func (s *Service) GoogleContext(ctx context.Context, token, transport string) (GoogleContext, error) {
	if _, err := s.googleAudience(transport); err != nil {
		return GoogleContext{}, err
	}
	flow, err := s.repo.readGoogleFlow(ctx, token, transport)
	return flow.GoogleContext, err
}

// ProveGoogle consumes the nonce/proof step once after cryptographic verification.
func (s *Service) ProveGoogle(ctx context.Context, token, transport, proof, source string) (GoogleResult, error) {
	if _, err := s.googleAudience(transport); err != nil {
		return GoogleResult{}, err
	}
	if err := s.googleAttempt(ctx, source); err != nil {
		return GoogleResult{}, err
	}
	flow, err := s.repo.readGoogleFlow(ctx, token, transport)
	if err != nil {
		return GoogleResult{}, err
	}
	if flow.Status != "awaiting_proof" {
		return GoogleResult{}, ErrInvalidFlow
	}
	if len(proof) == 0 || len(proof) > 16000 {
		return GoogleResult{}, ErrGoogleCredential
	}
	verifyCtx, cancel := context.WithTimeout(ctx, s.google.config.RequestTimeout)
	defer cancel()
	identity, err := s.google.verifier.Verify(verifyCtx, proof, flow.Audiences)
	if err != nil {
		return GoogleResult{}, ErrGoogleCredential
	}
	if subtle.ConstantTimeCompare([]byte(tokenHash(identity.Nonce)), []byte(flow.NonceHash)) != 1 {
		return GoogleResult{}, ErrGoogleCredential
	}
	// The repository rechecks flow state and expiry under lock after verification.
	return s.repo.advanceGoogle(ctx, token, transport, &identity, "", "", s, source)
}

// ExchangeGoogle exchanges a confirmed email flow for onboarding or a session.
func (s *Service) ExchangeGoogle(ctx context.Context, token, transport, source string) (GoogleResult, error) {
	if _, err := s.googleAudience(transport); err != nil {
		return GoogleResult{}, err
	}
	if err := s.googleAttempt(ctx, source); err != nil {
		return GoogleResult{}, err
	}
	return s.repo.advanceGoogle(ctx, token, transport, nil, "", "", s, source)
}

// CompleteGoogle creates credentials atomically, preserving active account passwords.
func (s *Service) CompleteGoogle(ctx context.Context, token, transport, username, password, source string) (GoogleResult, error) {
	if _, err := s.googleAudience(transport); err != nil {
		return GoogleResult{}, err
	}
	if !usernamePattern.MatchString(username) || ValidatePassword(password) != nil {
		return GoogleResult{}, ErrInvalidInput
	}
	if err := s.googleAttempt(ctx, source); err != nil {
		return GoogleResult{}, err
	}
	flow, err := s.repo.readGoogleFlow(ctx, token, transport)
	if err != nil {
		return GoogleResult{}, err
	}
	if flow.Status != "onboarding" {
		return GoogleResult{}, ErrInvalidFlow
	}
	if err = s.acquire(ctx); err != nil {
		return GoogleResult{}, err
	}
	defer func() { <-s.work }()
	return s.repo.advanceGoogle(ctx, token, transport, nil, username, hashPassword(password), s, source)
}

// ConfirmGoogleEmail consumes only the email challenge; it never returns a session.
func (s *Service) ConfirmGoogleEmail(ctx context.Context, token, source string) error {
	if s.google == nil {
		return ErrGoogleDisabled
	}
	if err := s.confirmAttempt(ctx, source); err != nil {
		return err
	}
	return s.repo.confirmGoogleEmail(ctx, token)
}

// ResendGoogleEmail replaces only this flow's challenge and keeps its original expiry.
func (s *Service) ResendGoogleEmail(ctx context.Context, token, transport, source string) error {
	if _, err := s.googleAudience(transport); err != nil {
		return err
	}
	if err := s.mailSource(ctx, source); err != nil {
		return err
	}
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	flow, err := loadGoogleFlow(ctx, tx, token, transport, true)
	if err != nil {
		return err
	}
	if flow.Status != "email_verification_required" {
		return ErrInvalidFlow
	}
	if err = s.repo.issueGoogleEmail(ctx, tx, flow, s.recovery); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func flowValidity(status string, expires time.Time) error {
	if status == "consumed" {
		return ErrInvalidFlow
	}
	if !expires.After(time.Now()) {
		return ErrFlowExpired
	}
	return nil
}
