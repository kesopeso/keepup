package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrGoogleCredential = errors.New("invalid_google_credential")
	ErrInvalidFlow      = errors.New("invalid_flow")
	ErrFlowExpired      = errors.New("flow_expired")
	ErrIdentityConflict = errors.New("identity_conflict")
	ErrGoogleDisabled   = errors.New("google_unavailable")
)

// GoogleConfig contains server-side client allowlists and the flow lifetime.
// NativeClientIDs are the expected ID-token audiences, which may be web client IDs.
type GoogleConfig struct {
	WebClientIDs    []string
	NativeClientIDs []string
	FlowTTL         time.Duration
	RequestTimeout  time.Duration
}

// DefaultGoogleConfig leaves Google disabled until client IDs are configured.
func DefaultGoogleConfig() GoogleConfig {
	return GoogleConfig{FlowTTL: 10 * time.Minute, RequestTimeout: 10 * time.Second}
}

// Validate rejects malformed client allowlists and invalid lifetimes.
func (c GoogleConfig) Validate() error {
	if c.FlowTTL <= 0 || c.RequestTimeout <= 0 {
		return fmt.Errorf("google lifetimes must be positive")
	}
	for _, ids := range [][]string{c.WebClientIDs, c.NativeClientIDs} {
		if len(ids) > 20 {
			return fmt.Errorf("at most 20 Google audiences per transport")
		}
		for _, id := range ids {
			prefix, ok := strings.CutSuffix(id, ".apps.googleusercontent.com")
			if !ok || prefix == "" || strings.ContainsAny(prefix, " /\t\r\n:@?#") {
				return fmt.Errorf("google audiences must be OAuth client IDs")
			}
		}
	}
	return nil
}

// GoogleIdentity contains only claims obtained from a verified identity proof.
type GoogleIdentity struct {
	Subject       string
	Email         string
	Nonce         string
	Authoritative bool
}

// GoogleProofVerifier isolates cryptographic verification from account decisions.
type GoogleProofVerifier interface {
	Verify(context.Context, string, []string) (GoogleIdentity, error)
}

// GoogleStart contains the nonce and an internal credential for transport delivery.
type GoogleStart struct {
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expiresAt"`
	Token     string    `json:"-"`
}

// GoogleContext is safe to return from polling; it never contains a credential.
type GoogleContext struct {
	Status    string    `json:"status"`
	Email     string    `json:"email,omitempty"`
	Username  string    `json:"username,omitempty"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// GoogleResult either advances onboarding or issues a session through the transport.
type GoogleResult struct {
	Context GoogleContext
	Account *Result
}

type googleRuntime struct {
	config   GoogleConfig
	verifier GoogleProofVerifier
}

type googleFlow struct {
	GoogleContext
	ID        string
	NonceHash string
	Subject   string
	Audiences []string
}
