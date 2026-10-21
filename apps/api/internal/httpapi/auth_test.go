package httpapi

import (
	"context"
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

type accountStub struct {
	calls int
	token string
	err   error
}

func (s *accountStub) result() (auth.Result, error) {
	s.calls++
	return auth.Result{Token: strings.Repeat("t", 43), Session: auth.Session{ID: "internal-session", User: auth.User{ID: "user", Email: "test@example.com", Username: "Test"}, ExpiresAt: time.Now().Add(time.Hour)}}, s.err
}
func (s *accountStub) Signup(context.Context, auth.SignupInput, string) (auth.Result, error) {
	return s.result()
}
func (s *accountStub) Login(context.Context, string, string, string) (auth.Result, error) {
	return s.result()
}
func (s *accountStub) Authenticate(_ context.Context, token string) (auth.Session, error) {
	s.token = token
	r, err := s.result()
	return r.Session, err
}
func (s *accountStub) Logout(_ context.Context, token string, _ bool) error {
	s.token = token
	s.calls++
	return s.err
}
func (s *accountStub) Rotate(context.Context, string) (auth.Result, error) { return s.result() }

func (s *accountStub) ResendVerification(context.Context, string, string) error {
	s.calls++
	return s.err
}
func (s *accountStub) ConfirmVerification(context.Context, string, string) error {
	s.calls++
	return s.err
}
func (s *accountStub) ForgotPassword(context.Context, string, string) error { s.calls++; return s.err }
func (s *accountStub) ResetPassword(context.Context, string, string, string) error {
	s.calls++
	return s.err
}
func (s *accountStub) ChangePassword(context.Context, string, string, string, string) (auth.Result, error) {
	return s.result()
}

func accountTestHandler(s *accountStub) http.Handler {
	return NewAccountHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), config.AuthConfig{Origins: []string{"https://keepup.example"}, Policy: auth.DefaultPolicy()}, "production", s)
}

func TestAccountTransport(t *testing.T) {
	for _, tc := range []struct {
		name    string
		headers map[string]string
		status  int
		native  bool
	}{
		{"browser", map[string]string{"Origin": "https://keepup.example", "X-KeepUp-Request": "1"}, 201, false},
		{"native", map[string]string{"X-KeepUp-Client": "native"}, 201, true},
		{"cross origin", map[string]string{"Origin": "https://evil.example", "X-KeepUp-Request": "1"}, 403, false},
		{"no origin", nil, 403, false},
		{"no csrf header", map[string]string{"Origin": "https://keepup.example"}, 403, false},
		{"native spoof with origin", map[string]string{"X-KeepUp-Client": "native", "Origin": "https://evil.example"}, 403, false},
		{"native spoof with cookie", map[string]string{"X-KeepUp-Client": "native", "Cookie": "__Host-keepup_session=secret"}, 403, false},
		{"native spoof with fetch metadata", map[string]string{"X-KeepUp-Client": "native", "Sec-Fetch-Site": "cross-site"}, 403, false},
		{"mixed credentials", map[string]string{"Origin": "https://keepup.example", "X-KeepUp-Request": "1", "Authorization": "Bearer secret"}, 403, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &accountStub{}
			handler := accountTestHandler(s)
			r := httptest.NewRequest("POST", "/auth/signup", strings.NewReader(`{"email":"test@example.com","username":"Test","password":"password1!"}`))
			r.Header.Set("Content-Type", "application/json")
			for key, value := range tc.headers {
				r.Header.Set(key, value)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("credentials may be cached")
			}
			if tc.status != 201 {
				if s.calls != 0 {
					t.Fatal("rejected transport reached service")
				}
				return
			}
			if strings.Contains(w.Body.String(), "internal-session") {
				t.Fatal("internal session ID exposed")
			}
			cookies := w.Result().Cookies()
			if tc.native {
				if len(cookies) != 0 || !strings.Contains(w.Body.String(), `"sessionToken"`) {
					t.Fatal("native transport mismatch")
				}
			} else {
				if strings.Contains(w.Body.String(), `"sessionToken"`) || strings.Contains(w.Body.String(), strings.Repeat("t", 43)) {
					t.Fatal("browser response exposed credential")
				}
				if len(cookies) != 1 {
					t.Fatal("missing cookie")
				}
				c := cookies[0]
				if c.Name != "__Host-keepup_session" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" || c.Expires.IsZero() {
					t.Fatal("unsafe cookie", c)
				}
			}
		})
	}
}

func TestAccountJSONAndErrors(t *testing.T) {
	for _, body := range []string{`{}`, `{"email":"a@example.com","unknown":true}`, `{} {}`, `{"password":"` + strings.Repeat("x", 4096) + `"}`} {
		s := &accountStub{}
		handler := accountTestHandler(s)
		r := httptest.NewRequest("POST", "/auth/signup", strings.NewReader(body))
		r.Header.Set("X-KeepUp-Client", "native")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		// Empty fields are service-validated, all other malformed bodies stop at transport.
		if body != "{}" && (w.Code != 400 || s.calls != 0) {
			t.Fatalf("malformed input reached service: %d", w.Code)
		}
	}
	for _, tc := range []struct {
		err    error
		status int
		reason string
	}{
		{auth.ErrCredentials, 401, "invalid_credentials"}, {auth.ErrSession, 401, "session_expired"},
		{auth.ErrEmailTaken, 409, "email_taken"}, {auth.ErrRateLimited, 429, "rate_limited"},
		{io.ErrUnexpectedEOF, 500, "internal_error"},
	} {
		s := &accountStub{err: tc.err}
		handler := accountTestHandler(s)
		r := httptest.NewRequest("POST", "/auth/login", strings.NewReader(`{"identifier":"Test","password":"wrong"}`))
		r.Header.Set("X-KeepUp-Client", "native")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.reason) {
			t.Fatal(w.Code, w.Body.String())
		}
		if tc.status == 429 && w.Header().Get("Retry-After") != "900" {
			t.Fatal("invalid retry interval")
		}
	}
}

func TestAccountCookieAndBearerLookup(t *testing.T) {
	for _, native := range []bool{false, true} {
		s := &accountStub{}
		handler := accountTestHandler(s)
		r := httptest.NewRequest("GET", "/auth/me", nil)
		if native {
			r.Header.Set("X-KeepUp-Client", "native")
			r.Header.Set("Authorization", "Bearer native-token")
		} else {
			r.AddCookie(&http.Cookie{Name: "__Host-keepup_session", Value: "cookie-token"})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		want := "cookie-token"
		if native {
			want = "native-token"
		}
		if w.Code != 200 || s.token != want {
			t.Fatal("incorrect authority", w.Code, s.token)
		}
	}
}
