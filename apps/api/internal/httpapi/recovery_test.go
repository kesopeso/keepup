package httpapi

import (
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"keepup/apps/api/internal/config"
)

func TestRecoveryTransportAndScannerGET(t *testing.T) {
	for _, tc := range []struct {
		path, body string
		status     int
	}{
		{"/auth/verification/confirm", `{"token":"synthetic"}`, 200},
		{"/auth/verification/resend", `{}`, 202},
		{"/auth/password/forgot", `{"identifier":"missing"}`, 202},
		{"/auth/password/reset", `{"token":"synthetic","password":"password1!"}`, 204},
		{"/auth/password/change", `{"currentPassword":"password1!","password":"new-password1!"}`, 200},
	} {
		t.Run(tc.path, func(t *testing.T) {
			stub := &accountStub{}
			handler := accountTestHandler(stub)
			scanner := httptest.NewRequest("GET", tc.path+"?token=synthetic", nil)
			sw := httptest.NewRecorder()
			handler.ServeHTTP(sw, scanner)
			if sw.Code != 405 || stub.calls != 0 || len(sw.Result().Cookies()) != 0 {
				t.Fatal("scanner GET had side effects", sw.Code, stub.calls)
			}
			r := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-KeepUp-Client", "native")
			r.Header.Set("Authorization", "Bearer synthetic-session")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.path == "/auth/verification/confirm" || tc.path == "/auth/password/reset" {
				if strings.Contains(w.Body.String(), "sessionToken") || len(w.Result().Cookies()) != 0 {
					t.Fatal("link opener received a session")
				}
			}
			// The API must not silently accept a confirmation field the client forgot to check.
			malformed := httptest.NewRequest("POST", tc.path, strings.NewReader(`{"confirmPassword":"mismatch"}`))
			malformed.Header.Set("Content-Type", "application/json")
			malformed.Header.Set("X-KeepUp-Client", "native")
			mw := httptest.NewRecorder()
			handler.ServeHTTP(mw, malformed)
			if mw.Code != 400 {
				t.Fatal("unknown credential fields accepted", mw.Code)
			}
		})
	}
}

func TestTrustedProxySource(t *testing.T) {
	h := &accountHandler{cfg: config.AuthConfig{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}}}
	for _, tc := range []struct{ peer, forwarded, want string }{
		{"192.0.2.1:1234", "198.51.100.8", "192.0.2.1"},
		{"10.0.0.2:1234", "198.51.100.8", "198.51.100.8"},
		{"10.0.0.2:1234", "192.0.2.66, 198.51.100.8, 10.0.0.3", "198.51.100.8"},
		{"10.0.0.2:1234", "invalid", "10.0.0.2"},
		{"[::ffff:192.0.2.1]:1234", "198.51.100.8", "192.0.2.1"},
	} {
		r := httptest.NewRequest("POST", "/auth/login", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.forwarded)
		if got := h.sourceAddress(r); got != tc.want {
			t.Fatalf("source=%s want=%s", got, tc.want)
		}
	}
}
