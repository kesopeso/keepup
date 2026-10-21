package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/config"
)

type googleStub struct {
	accountStub
	authenticated bool
	transport     string
}

func (s *googleStub) StartGoogle(_ context.Context, transport, _ string) (auth.GoogleStart, error) {
	s.calls++
	s.transport = transport
	return auth.GoogleStart{Nonce: "server-nonce", Token: strings.Repeat("f", 43), ExpiresAt: time.Now().Add(time.Minute)}, s.err
}
func (s *googleStub) googleResult(token, transport string) (auth.GoogleResult, error) {
	s.token = token
	s.transport = transport
	result, err := s.result()
	if s.authenticated {
		return auth.GoogleResult{Account: &result}, err
	}
	return auth.GoogleResult{Context: auth.GoogleContext{Status: "onboarding", Email: "User@gmail.com", ExpiresAt: time.Now().Add(time.Minute)}}, err
}
func (s *googleStub) ProveGoogle(_ context.Context, token, transport, _, _ string) (auth.GoogleResult, error) {
	return s.googleResult(token, transport)
}
func (s *googleStub) CompleteGoogle(_ context.Context, token, transport, _, _, _ string) (auth.GoogleResult, error) {
	return s.googleResult(token, transport)
}
func (s *googleStub) ExchangeGoogle(_ context.Context, token, transport, _ string) (auth.GoogleResult, error) {
	return s.googleResult(token, transport)
}
func (s *googleStub) GoogleContext(_ context.Context, token, transport string) (auth.GoogleContext, error) {
	s.calls++
	s.token = token
	s.transport = transport
	return auth.GoogleContext{Status: "ready", ExpiresAt: time.Now().Add(time.Minute)}, s.err
}
func (s *googleStub) ConfirmGoogleEmail(context.Context, string, string) error {
	s.calls++
	return s.err
}
func (s *googleStub) ResendGoogleEmail(_ context.Context, token, transport, _ string) error {
	s.calls++
	s.token = token
	s.transport = transport
	return s.err
}

func googleHandler(s *googleStub) http.Handler {
	return NewAccountHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), config.AuthConfig{Origins: []string{"https://keepup.example"}}, "production", s)
}

func googleRequest(handler http.Handler, method, path, body string, native bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if native {
		r.Header.Set("X-KeepUp-Client", "native")
		r.Header.Set("Authorization", "Bearer flow-credential")
	} else {
		r.Header.Set("Origin", "https://keepup.example")
		r.Header.Set("X-KeepUp-Request", "1")
		r.AddCookie(&http.Cookie{Name: "__Host-keepup_session", Value: "account-credential"})
		r.AddCookie(&http.Cookie{Name: "__Host-keepup_google", Value: "flow-credential"})
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestGoogleFlowTransport(t *testing.T) {
	for _, native := range []bool{false, true} {
		s := &googleStub{}
		handler := googleHandler(s)
		start := googleRequest(handler, "POST", "/auth/google/start", "{}", native)
		if start.Code != 200 {
			t.Fatal(start.Code, start.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(start.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if native {
			if response["flowToken"] == nil || len(start.Result().Cookies()) != 0 {
				t.Fatal("native credential missing")
			}
		} else {
			if response["flowToken"] != nil || strings.Contains(start.Body.String(), strings.Repeat("f", 43)) {
				t.Fatal("browser flow secret leaked")
			}
			cookies := start.Result().Cookies()
			if len(cookies) != 1 || cookies[0].Name != "__Host-keepup_google" || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].Domain != "" || cookies[0].Path != "/" || cookies[0].SameSite != http.SameSiteLaxMode {
				t.Fatalf("bad flow cookie %+v", cookies)
			}
		}
		for _, tc := range []struct {
			method, path, body string
			status             int
		}{
			{"POST", "/auth/google/proof", `{"idToken":"proof"}`, 200},
			{"POST", "/auth/google/complete", `{"username":"Test","password":"password1!"}`, 200},
			{"POST", "/auth/google/exchange", `{}`, 200},
			{"GET", "/auth/google/context", ``, 200},
			{"POST", "/auth/google/email/resend", `{}`, 202},
		} {
			w := googleRequest(handler, tc.method, tc.path, tc.body, native)
			if w.Code != tc.status || s.token != "flow-credential" {
				t.Fatal(tc.path, w.Code, s.token, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "sessionToken") || len(w.Result().Cookies()) != 0 {
				t.Fatal("non-session step issued credentials")
			}
		}
		confirm := googleRequest(handler, "POST", "/auth/google/email/confirm", `{"token":"email-token"}`, native)
		if confirm.Code != 200 || len(confirm.Result().Cookies()) != 0 {
			t.Fatal("confirmation signed in opener")
		}
		s.authenticated = true
		signedIn := googleRequest(handler, "POST", "/auth/google/proof", `{"idToken":"proof"}`, native)
		if signedIn.Code != 200 || !strings.Contains(signedIn.Body.String(), `"status":"authenticated"`) {
			t.Fatal(signedIn.Body.String())
		}
		if native {
			if !strings.Contains(signedIn.Body.String(), `"sessionToken"`) || len(signedIn.Result().Cookies()) != 0 {
				t.Fatal("native session transport")
			}
		} else {
			if strings.Contains(signedIn.Body.String(), strings.Repeat("t", 43)) {
				t.Fatal("browser session leaked")
			}
			cookies := signedIn.Result().Cookies()
			if len(cookies) != 2 || cookies[0].Name != "__Host-keepup_session" || cookies[1].Name != "__Host-keepup_google" || cookies[1].MaxAge != -1 {
				t.Fatal("flow not cleared", cookies)
			}
		}
	}
}

func TestGoogleRequestGuards(t *testing.T) {
	s := &googleStub{}
	handler := googleHandler(s)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/auth/google/email/confirm?token=scanner", "", 405},
		{"GET", "/auth/google/proof?token=scanner", "", 405},
		{"POST", "/auth/google/proof", `{"idToken":"p","email":"attacker@example.com"}`, 400},
		{"POST", "/auth/google/proof", `{"idToken":"p"} {}`, 400},
		{"POST", "/auth/google/proof", `{"idToken":"` + strings.Repeat("x", 16400) + `"}`, 400},
		{"POST", "/auth/google/complete", `{"username":"Test","password":"password1!","confirmPassword":"password1!"}`, 400},
	} {
		w := googleRequest(handler, tc.method, tc.path, tc.body, false)
		if w.Code != tc.status || s.calls != 0 {
			t.Fatal(tc.path, w.Code, s.calls, w.Body.String())
		}
	}
	// Google proofs have a larger body limit than password forms.
	w := googleRequest(handler, "POST", "/auth/google/proof", `{"idToken":"`+strings.Repeat("x", 6000)+`"}`, false)
	if w.Code != 200 || s.calls != 1 {
		t.Fatal("valid sized proof rejected", w.Code)
	}
	r := httptest.NewRequest("POST", "/auth/google/start", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://evil.example")
	r.Header.Set("X-KeepUp-Request", "1")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 || s.calls != 1 {
		t.Fatal("origin guard failed")
	}
	for _, tc := range []struct {
		err    error
		status int
	}{
		{auth.ErrGoogleCredential, 401}, {auth.ErrInvalidFlow, 400}, {auth.ErrFlowExpired, 400}, {auth.ErrIdentityConflict, 409}, {auth.ErrGoogleDisabled, 503},
	} {
		s.err = tc.err
		w = googleRequest(handler, "POST", "/auth/google/start", "{}", false)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.err.Error()) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
