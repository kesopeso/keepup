package auth

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Policy contains configurable authentication limits. Zero values use defaults.
type Policy struct {
	SessionIdle    time.Duration
	AttemptWindow  time.Duration
	LoginAttempts  int
	SignupAttempts int
}

// DefaultPolicy returns the initial deployment defaults.
func DefaultPolicy() Policy { return Policy{30 * 24 * time.Hour, 15 * time.Minute, 20, 5} }

// Service bounds expensive password operations and delegates authority to PostgreSQL.
type Service struct {
	repo      *PostgresRepository
	policy    Policy
	work      chan struct{}
	dummyHash string
	recovery  *recoveryRuntime
	google    *googleRuntime
}

// ServiceOption configures an optional identity provider before serving requests.
type ServiceOption func(*Service) error

// WithGoogle enables server-bound Google flows using an injected proof verifier.
func WithGoogle(cfg GoogleConfig, verifier GoogleProofVerifier) ServiceOption {
	return func(s *Service) error {
		if err := cfg.Validate(); err != nil {
			return err
		}
		if verifier == nil {
			return errors.New("google verifier is required")
		}
		cfg.WebClientIDs = append([]string(nil), cfg.WebClientIDs...)
		cfg.NativeClientIDs = append([]string(nil), cfg.NativeClientIDs...)
		s.google = &googleRuntime{config: cfg, verifier: verifier}
		return nil
	}
}

// NewService constructs account authority using a validated, nonzero policy.
func NewService(repo *PostgresRepository, policy Policy, recovery RecoveryConfig, options ...ServiceOption) (*Service, error) {
	runtime, err := newRecovery(recovery)
	if err != nil {
		return nil, err
	}
	defaults := DefaultPolicy()
	if policy.SessionIdle <= 0 {
		policy.SessionIdle = defaults.SessionIdle
	}
	if policy.AttemptWindow <= 0 {
		policy.AttemptWindow = defaults.AttemptWindow
	}
	if policy.LoginAttempts <= 0 {
		policy.LoginAttempts = defaults.LoginAttempts
	}
	if policy.SignupAttempts <= 0 {
		policy.SignupAttempts = defaults.SignupAttempts
	}
	if recovery.Policy.Retention < policy.AttemptWindow {
		return nil, errors.New("retention must cover authentication attempt window")
	}
	s := &Service{repo: repo, policy: policy, work: make(chan struct{}, 2), dummyHash: hashPassword(newToken()), recovery: runtime}
	for _, option := range options {
		if err := option(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Rotate replaces the credential atomically without changing session identity.
// There is no overlap period: callers must serialize rotation with their requests.
func (s *Service) Rotate(ctx context.Context, oldToken string) (Result, error) {
	if len(oldToken) != 43 {
		return Result{}, ErrSession
	}
	token := newToken()
	session, err := s.repo.rotate(ctx, tokenHash(oldToken), tokenHash(token), s.policy.SessionIdle)
	if err != nil {
		return Result{}, err
	}
	return Result{session, token}, nil
}

func (s *Service) acquire(ctx context.Context) error {
	select {
	case s.work <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Signup atomically creates a pending account and a restricted session.
func (s *Service) Signup(ctx context.Context, input SignupInput, source string) (Result, error) {
	if err := ValidateSignup(input); err != nil {
		return Result{}, err
	}
	if err := s.repo.allowAttempt(ctx, "signup:source:"+source, s.policy.AttemptWindow, s.policy.SignupAttempts); err != nil {
		return Result{}, err
	}
	if err := s.mailSource(ctx, source); err != nil {
		return Result{}, err
	}
	if err := s.acquire(ctx); err != nil {
		return Result{}, err
	}
	defer func() { <-s.work }()
	input.Email = strings.TrimSpace(input.Email)
	token := newToken()
	session, err := s.repo.signup(ctx, input, hashPassword(input.Password), tokenHash(token), s.policy.SessionIdle, s.recovery)
	if err != nil {
		return Result{}, err
	}
	return Result{session, token}, nil
}

// Login accepts a case-insensitive email or username. Wrong and unknown credentials match.
func (s *Service) Login(ctx context.Context, identifier, password, source string) (Result, error) {
	identifier = NormalizeIdentifier(identifier)
	if len(identifier) > 254 || !utf8.ValidString(password) || utf8.RuneCountInString(password) > 128 {
		return Result{}, ErrCredentials
	}
	for _, key := range []string{"login:source:" + source, "login:identifier:" + identifier} {
		if err := s.repo.allowAttempt(ctx, key, s.policy.AttemptWindow, s.policy.LoginAttempts); err != nil {
			return Result{}, err
		}
	}
	if err := s.acquire(ctx); err != nil {
		return Result{}, err
	}
	defer func() { <-s.work }()
	id, hash, err := s.repo.password(ctx, identifier)
	if err != nil && !errors.Is(err, ErrCredentials) {
		return Result{}, err
	}
	if errors.Is(err, ErrCredentials) {
		hash = s.dummyHash
	}
	if !matchesPassword(password, hash) || id == "" {
		return Result{}, ErrCredentials
	}
	token := newToken()
	session, err := s.repo.login(ctx, id, hash, tokenHash(token), s.policy.SessionIdle)
	if err != nil {
		return Result{}, err
	}
	return Result{session, token}, nil
}

// Authenticate refreshes idle expiry only for an unexpired, unrevoked credential.
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	if len(token) != 43 {
		return Session{}, ErrSession
	}
	return s.repo.authenticate(ctx, tokenHash(token), s.policy.SessionIdle)
}

// RequireActive is the shared gate for future account-bound route services.
func RequireActive(session Session) error {
	if !session.User.Verified {
		return ErrPending
	}
	return nil
}

// Logout revokes current or all account sessions. B6 adds account socket cleanup.
func (s *Service) Logout(ctx context.Context, token string, everywhere bool) error {
	if len(token) != 43 {
		return ErrSession
	}
	return s.repo.logout(ctx, tokenHash(token), everywhere)
}
