package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

func signGoogle(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "test"))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestGoogleVerifier(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise the real remote-key path against a local provider, including key caching.
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}}); err != nil {
			t.Error(err)
		}
	}))
	defer provider.Close()
	verifier := newGoogleVerifier(context.Background(), provider.URL, provider.Client(), []string{"web.apps.googleusercontent.com", "android.apps.googleusercontent.com"})
	for _, tc := range []struct {
		name          string
		change        func(map[string]any)
		otherKey      bool
		valid         bool
		authoritative bool
	}{
		{name: "gmail", valid: true, authoritative: true},
		{name: "legacy issuer", change: func(c map[string]any) { c["iss"] = "accounts.google.com" }, valid: true, authoritative: true},
		{name: "workspace", change: func(c map[string]any) { c["email"] = "User@company.example"; c["hd"] = "company.example" }, valid: true, authoritative: true},
		{name: "third party", change: func(c map[string]any) { c["email"] = "User@third.example" }, valid: true},
		{name: "bad issuer", change: func(c map[string]any) { c["iss"] = "https://attacker.example" }},
		{name: "bad audience", change: func(c map[string]any) { c["aud"] = "evil.apps.googleusercontent.com" }},
		{name: "bad presenter", change: func(c map[string]any) { c["azp"] = "evil.apps.googleusercontent.com" }},
		{name: "native presenter", change: func(c map[string]any) { c["azp"] = "android.apps.googleusercontent.com" }, valid: true, authoritative: true},
		{name: "expired", change: func(c map[string]any) { c["exp"] = time.Now().Add(-time.Second).Unix() }},
		{name: "future issuance", change: func(c map[string]any) { c["iat"] = time.Now().Add(time.Hour).Unix() }},
		{name: "unverified", change: func(c map[string]any) { c["email_verified"] = false }},
		{name: "no subject", change: func(c map[string]any) { delete(c, "sub") }},
		{name: "missing nonce", change: func(c map[string]any) { delete(c, "nonce") }},
		{name: "bad email", change: func(c map[string]any) { c["email"] = "Display <user@gmail.com>" }},
		{name: "bad signature", otherKey: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := map[string]any{"iss": "https://accounts.google.com", "sub": "subject-1", "aud": "web.apps.googleusercontent.com", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "email": "User@gmail.com", "email_verified": true, "nonce": "nonce"}
			if tc.change != nil {
				tc.change(claims)
			}
			signingKey := key
			if tc.otherKey {
				signingKey = other
			}
			identity, err := verifier.Verify(context.Background(), signGoogle(t, signingKey, claims), []string{"web.apps.googleusercontent.com"})
			if tc.valid {
				if err != nil || identity.Authoritative != tc.authoritative || identity.Subject != "subject-1" {
					t.Fatalf("identity=%+v err=%v", identity, err)
				}
			} else if !errors.Is(err, ErrGoogleCredential) {
				t.Fatalf("invalid proof accepted: %v", err)
			}
		})
	}
}
