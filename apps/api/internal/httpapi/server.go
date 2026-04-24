// Package httpapi provides the HTTP surface for the KeepUp API.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"keepup/apps/api/internal/config"
	"keepup/apps/api/internal/routes"
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
	routes    RouteService
}

// RouteService contains the first route lifecycle operations served over HTTP.
type RouteService interface {
	CreateRoute(context.Context, routes.CreateRouteInput) (routes.CreateRouteResult, error)
	AccessRoute(context.Context, string) (routes.AccessRouteResult, error)
	JoinRoute(context.Context, string, routes.JoinRouteInput) (routes.JoinRouteResult, error)
	Snapshot(context.Context, string, string) (routes.Snapshot, error)
	UpdateRoute(context.Context, string, string, routes.UpdateRouteInput) (routes.Route, error)
	LeaveRoute(context.Context, string, string) (routes.LeaveRouteResult, error)
	DeleteRoute(context.Context, string, string) error
}

// NewHandler builds the KeepUp API HTTP handler tree.
func NewHandler(logger *slog.Logger, cfg config.AppConfig, db HealthChecker, routeService RouteService) http.Handler {
	server := &Server{
		appConfig: cfg,
		db:        db,
		logger:    logger,
		routes:    routeService,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleRoot)
	mux.HandleFunc("/livez", server.handleLiveness)
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("POST /routes", server.handleCreateRoute)
	mux.HandleFunc("GET /routes/{code}/access", server.handleRouteAccess)
	mux.HandleFunc("POST /routes/{code}/members", server.handleCreateRouteMember)
	mux.HandleFunc("GET /routes/{code}", server.handleRoute)
	mux.HandleFunc("PATCH /routes/{code}", server.handleUpdateRoute)
	mux.HandleFunc("DELETE /routes/{code}", server.handleDeleteRoute)
	mux.HandleFunc("DELETE /routes/{code}/members/me", server.handleLeaveRoute)

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

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ClientID      string `json:"clientId"`
		DisplayName   string `json:"displayName"`
		TransportMode string `json:"transportMode"`
		Name          string `json:"name"`
		Description   string `json:"description"`
		Password      string `json:"password"`
		SharingPolicy string `json:"sharingPolicy"`
	}

	if err := decodeJSON(r.Body, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	result, err := s.routes.CreateRoute(r.Context(), routes.CreateRouteInput{
		ClientID:      request.ClientID,
		DisplayName:   request.DisplayName,
		TransportMode: request.TransportMode,
		Name:          request.Name,
		Description:   request.Description,
		Password:      request.Password,
		SharingPolicy: request.SharingPolicy,
	})
	if err != nil {
		s.writeRouteError(w, err)
		return
	}

	s.writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleRouteAccess(w http.ResponseWriter, r *http.Request) {
	result, err := s.routes.AccessRoute(r.Context(), r.PathValue("code"))
	if err != nil {
		s.writeRouteError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleCreateRouteMember(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ClientID      string `json:"clientId"`
		DisplayName   string `json:"displayName"`
		TransportMode string `json:"transportMode"`
		Password      string `json:"password"`
	}

	if err := decodeJSON(r.Body, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	result, err := s.routes.JoinRoute(r.Context(), r.PathValue("code"), routes.JoinRouteInput{
		ClientID:      request.ClientID,
		DisplayName:   request.DisplayName,
		TransportMode: request.TransportMode,
		Password:      request.Password,
	})
	if err != nil {
		s.writeRouteError(w, err)
		return
	}

	s.writeJSON(w, http.StatusCreated, result)
}

func (s *Server) handleRoute(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		s.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	result, err := s.routes.Snapshot(r.Context(), r.PathValue("code"), token)
	if err != nil {
		s.writeRouteError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		s.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var request struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}

	if err := decodeJSON(r.Body, &request); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}

	result, err := s.routes.UpdateRoute(r.Context(), r.PathValue("code"), token, routes.UpdateRouteInput{
		Name:        request.Name,
		Description: request.Description,
		Status:      request.Status,
	})
	if err != nil {
		s.writeRouteError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleLeaveRoute(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		s.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	result, err := s.routes.LeaveRoute(r.Context(), r.PathValue("code"), token)
	if err != nil {
		s.writeRouteError(w, err)
		return
	}

	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		s.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := s.routes.DeleteRoute(r.Context(), r.PathValue("code"), token); err != nil {
		s.writeRouteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		s.logger.Error("failed to write json response", "error", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, reason string) {
	s.writeJSON(w, status, map[string]string{"error": reason})
}

func (s *Server) writeRouteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, routes.ErrInvalidInput):
		s.writeError(w, http.StatusBadRequest, "invalid_input")
	case errors.Is(err, routes.ErrRouteNotFound):
		s.writeError(w, http.StatusNotFound, "route_not_found")
	case errors.Is(err, routes.ErrInvalidPassword):
		s.writeError(w, http.StatusUnauthorized, "invalid_password")
	case errors.Is(err, routes.ErrAliasTaken):
		s.writeError(w, http.StatusConflict, "alias_taken")
	case errors.Is(err, routes.ErrUnauthorized):
		s.writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, routes.ErrRouteClosed):
		s.writeError(w, http.StatusConflict, "route_closed")
	default:
		s.logger.Error("route handler failed", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal_error")
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

func decodeJSON(body io.ReadCloser, dst any) error {
	defer body.Close()

	if err := json.NewDecoder(body).Decode(dst); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	return nil
}

func bearerToken(header string) string {
	const prefix = "Bearer "

	if !strings.HasPrefix(header, prefix) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
