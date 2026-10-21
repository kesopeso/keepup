// Package auth owns accounts, password credentials, and revocable sessions.
package auth

import (
	"errors"
	"time"
)

// Errors are safe API reason codes. Database and credential details stay internal.
var (
	ErrInvalidInput  = errors.New("invalid_input")
	ErrEmailTaken    = errors.New("email_taken")
	ErrUsernameTaken = errors.New("username_taken")
	ErrCredentials   = errors.New("invalid_credentials")
	ErrSession       = errors.New("session_expired")
	ErrPending       = errors.New("email_verification_required")
	ErrRateLimited   = errors.New("rate_limited")
	ErrInvalidToken  = errors.New("invalid_token")
	ErrTokenExpired  = errors.New("token_expired")
)

// RateLimitError supplies a conservative retry delay for the applicable policy.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string { return ErrRateLimited.Error() }

// Unwrap preserves stable rate_limited matching across transports.
func (e *RateLimitError) Unwrap() error { return ErrRateLimited }

// User exposes immutable identifiers and activation state, never a password hash.
type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Verified bool   `json:"verified"`
}

// Session identifies server-side authority, independently of the raw credential.
type Session struct {
	ID        string    `json:"-"`
	User      User      `json:"user"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Result returns a newly issued credential to the transport layer only.
type Result struct {
	Session Session
	Token   string
}

// SignupInput contains email signup fields. Confirmation is checked by clients.
type SignupInput struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}
