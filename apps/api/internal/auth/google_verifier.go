package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// GoogleVerifier verifies Google's signatures and standard OIDC claims using go-oidc.
// The remote key set caches public keys and refreshes them on key rotation.
type GoogleVerifier struct {
	verifier   *oidc.IDTokenVerifier
	presenters []string
}

// NewGoogleVerifier uses Google's fixed HTTPS key endpoint, with bounded requests.
func NewGoogleVerifier(ctx context.Context, timeout time.Duration, presenters []string) *GoogleVerifier {
	return newGoogleVerifier(ctx, "https://www.googleapis.com/oauth2/v3/certs", &http.Client{Timeout: timeout}, presenters)
}

func newGoogleVerifier(ctx context.Context, endpoint string, client *http.Client, presenters []string) *GoogleVerifier {
	keys := oidc.NewRemoteKeySet(oidc.ClientContext(ctx, client), endpoint)
	// Audience validation is performed below against the flow's stored transport allowlist.
	return &GoogleVerifier{verifier: oidc.NewVerifier("https://accounts.google.com", keys, &oidc.Config{SkipClientIDCheck: true, SupportedSigningAlgs: []string{"RS256"}}), presenters: slices.Clone(presenters)}
}

// Verify never accepts profile JSON or unverified decoded claims as identity.
func (v *GoogleVerifier) Verify(ctx context.Context, raw string, audiences []string) (GoogleIdentity, error) {
	token, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return GoogleIdentity{}, ErrGoogleCredential
	}
	if len(token.Audience) == 0 || token.Subject == "" || len(token.Subject) > 255 || token.Nonce == "" || !token.Expiry.After(time.Now()) || token.IssuedAt.IsZero() || token.IssuedAt.After(time.Now().Add(time.Minute)) {
		return GoogleIdentity{}, ErrGoogleCredential
	}
	for _, aud := range token.Audience {
		if !slices.Contains(audiences, aud) {
			return GoogleIdentity{}, ErrGoogleCredential
		}
	}
	var claims struct {
		Email        string `json:"email"`
		Verified     bool   `json:"email_verified"`
		HostedDomain string `json:"hd"`
		Presenter    string `json:"azp"`
	}
	if err = token.Claims(&claims); err != nil || !claims.Verified || (len(token.Audience) > 1 && claims.Presenter == "") || (claims.Presenter != "" && !slices.Contains(v.presenters, claims.Presenter)) {
		return GoogleIdentity{}, ErrGoogleCredential
	}
	if ValidateSignup(SignupInput{Email: claims.Email, Username: "Valid", Password: "validpass1!"}) != nil {
		return GoogleIdentity{}, ErrGoogleCredential
	}
	claims.Email = strings.TrimSpace(claims.Email)
	domain := claims.Email[strings.LastIndex(claims.Email, "@")+1:]
	return GoogleIdentity{Subject: token.Subject, Email: claims.Email, Nonce: token.Nonce, Authoritative: strings.EqualFold(domain, "gmail.com") || claims.HostedDomain != ""}, nil
}
