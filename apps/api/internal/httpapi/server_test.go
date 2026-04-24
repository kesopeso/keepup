package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"keepup/apps/api/internal/config"
)

type stubHealthChecker struct {
	err error
}

func (s stubHealthChecker) Ping(_ context.Context) error {
	return s.err
}

func TestHealthAndLivenessHandlers(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		path      string
		healthErr error
		wantCode  int
		wantBody  string
	}{
		{
			name:     "liveness is always ok",
			path:     "/livez",
			wantCode: http.StatusOK,
			wantBody: `"status":"ok"`,
		},
		{
			name:     "health returns ok when database is ready",
			path:     "/healthz",
			wantCode: http.StatusOK,
			wantBody: `"status":"ok"`,
		},
		{
			name:      "health returns service unavailable when database is down",
			path:      "/healthz",
			healthErr: errors.New("db down"),
			wantCode:  http.StatusServiceUnavailable,
			wantBody:  `"reason":"database_unavailable"`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := NewHandler(
				slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
				config.AppConfig{Env: "test", Port: "8080"},
				stubHealthChecker{err: tc.healthErr},
			)

			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantCode {
				t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, tc.wantCode)
			}

			if body := recorder.Body.String(); !strings.Contains(body, tc.wantBody) {
				t.Fatalf("ServeHTTP() body = %q, want substring %q", body, tc.wantBody)
			}
		})
	}
}

func TestRootHandler(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
	)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if body := recorder.Body.String(); !strings.Contains(body, `"env":"test"`) {
		t.Fatalf("ServeHTTP() body = %q, want env field", body)
	}
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}
