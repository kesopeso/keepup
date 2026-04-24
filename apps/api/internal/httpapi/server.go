// Package httpapi provides the HTTP surface for the KeepUp API.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"keepup/apps/api/internal/config"
)

const healthCheckTimeout = 2 * time.Second

// HealthChecker reports whether the API dependencies are reachable.
type HealthChecker interface {
	Ping(context.Context) error
}

// Server contains the HTTP handler dependencies.
type Server struct {
	appConfig config.AppConfig
	db        HealthChecker
	logger    *slog.Logger
}

// NewHandler builds the KeepUp API HTTP handler tree.
func NewHandler(logger *slog.Logger, cfg config.AppConfig, db HealthChecker) http.Handler {
	server := &Server{
		appConfig: cfg,
		db:        db,
		logger:    logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/livez", server.handleLiveness)
	mux.HandleFunc("/healthz", server.handleHealth)

	return server.withCORS(server.withLogging(mux))
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"name":    "KeepUp API",
		"status":  "ok",
		"version": "dev",
		"env":     s.appConfig.Env,
	})
}

func (s *Server) handleLiveness(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	checkCtx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	if err := s.db.Ping(checkCtx); err != nil {
		s.logger.Error("database health check failed", "error", err)
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "degraded",
			"reason": "database_unavailable",
		})
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		s.logger.Error("failed to write json response", "error", err)
	}
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		next.ServeHTTP(w, r)
	})
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Serve starts the HTTP server and blocks until it exits.
func Serve(ctx context.Context, logger *slog.Logger, cfg config.AppConfig, handler http.Handler) error {
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		logger.Info("starting api server", "port", cfg.Port, "env", cfg.Env)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("listen and serve: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}

		if err := <-errCh; err != nil {
			return err
		}

		return nil
	case err := <-errCh:
		return err
	}
}
