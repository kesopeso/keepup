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
	"keepup/apps/api/internal/live"
	"keepup/apps/api/internal/routes"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

const healthCheckTimeout = 2 * time.Second
const defaultWebSocketAuthTimeout = 5 * time.Second

// HealthChecker reports whether the API dependencies are reachable.
type HealthChecker interface {
	Ping(context.Context) error
}

// Server contains the HTTP handler dependencies.
type Server struct {
	appConfig config.AppConfig
	db        HealthChecker
	liveHub   *live.Hub
	logger    *slog.Logger
	routes    RouteService
}

// RouteService contains the first route lifecycle operations served over HTTP.
type RouteService interface {
	CreateRoute(context.Context, routes.CreateRouteInput) (routes.CreateRouteResult, error)
	AccessRoute(context.Context, string) (routes.AccessRouteResult, error)
	AuthorizeMember(context.Context, string) (routes.AuthorizedMember, error)
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
		liveHub:   live.NewHub(),
		logger:    logger,
		routes:    routeService,
	}
	if server.appConfig.WebSocketAuthTimeout <= 0 {
		server.appConfig.WebSocketAuthTimeout = defaultWebSocketAuthTimeout
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
	mux.HandleFunc("GET /ws", server.handleWebSocket)

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

	s.broadcastLiveEvent(result.Route.ID, live.Event{
		"type":   "member_joined",
		"member": result.Member,
	})
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

	eventType := "route_updated"
	if result.Status == routes.RouteStatusClosed {
		eventType = "route_closed"
	}
	s.broadcastLiveEvent(result.ID, live.Event{
		"type":  eventType,
		"route": result,
	})
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

	s.broadcastLiveEvent(result.Member.RouteID, live.Event{
		"type":   "member_left",
		"member": result.Member,
	})
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

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		s.logger.Error("websocket accept failed", "error", err)
		return
	}
	defer func() {
		if err := connection.Close(websocket.StatusNormalClosure, "closing"); err != nil {
			s.logger.Debug("websocket close failed", "error", err)
		}
	}()

	authMessage, err := readWebSocketAuth(r.Context(), connection, s.appConfig.WebSocketAuthTimeout)
	if err != nil {
		s.logger.Info("websocket authentication failed", "error", err)
		_ = connection.Close(websocket.StatusPolicyViolation, "authentication required")
		return
	}

	authorized, err := s.routes.AuthorizeMember(r.Context(), authMessage.MemberToken)
	if err != nil {
		s.logger.Info("websocket token rejected", "error", err)
		_ = connection.Close(websocket.StatusPolicyViolation, "unauthorized")
		return
	}

	subscription := s.liveHub.Subscribe(authorized.Route.ID, authorized.Member.ID)
	defer subscription.Close()

	s.logger.Info("websocket subscribed",
		"route_id", authorized.Route.ID,
		"route_code", authorized.Route.Code,
		"member_id", authorized.Member.ID,
		"connections", s.liveHub.RouteConnectionCount(authorized.Route.ID),
	)

	if err := writeWebSocketJSON(r.Context(), connection, map[string]any{
		"type": "connection_established",
		"route": map[string]any{
			"id":     authorized.Route.ID,
			"code":   authorized.Route.Code,
			"status": authorized.Route.Status,
		},
		"member": map[string]any{
			"id":     authorized.Member.ID,
			"status": authorized.Member.Status,
		},
	}); err != nil {
		s.logger.Debug("websocket initial write failed", "error", err)
		return
	}

	readErrCh := make(chan error, 1)
	go func() {
		for {
			var message map[string]any
			if err := wsjson.Read(r.Context(), connection, &message); err != nil {
				readErrCh <- err
				return
			}
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case err := <-readErrCh:
			s.logger.Debug("websocket read loop ended", "error", err)
			return
		case event, ok := <-subscription.Events():
			if !ok {
				return
			}
			if err := writeWebSocketJSON(r.Context(), connection, event); err != nil {
				s.logger.Debug("websocket event write failed", "error", err)
				return
			}
		}
	}
}

func (s *Server) broadcastLiveEvent(routeID string, event live.Event) {
	if strings.TrimSpace(routeID) == "" {
		return
	}

	delivered := s.liveHub.Broadcast(routeID, event)
	if delivered > 0 {
		s.logger.Debug("broadcast live event", "route_id", routeID, "type", event["type"], "delivered", delivered)
	}
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

type webSocketAuthMessage struct {
	Type        string `json:"type"`
	MemberToken string `json:"memberToken"`
}

func readWebSocketAuth(ctx context.Context, connection *websocket.Conn, timeout time.Duration) (webSocketAuthMessage, error) {
	authCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var message webSocketAuthMessage
	if err := wsjson.Read(authCtx, connection, &message); err != nil {
		return webSocketAuthMessage{}, fmt.Errorf("read auth message: %w", err)
	}

	if message.Type != "authenticate" || strings.TrimSpace(message.MemberToken) == "" {
		return webSocketAuthMessage{}, routes.ErrUnauthorized
	}

	message.MemberToken = strings.TrimSpace(message.MemberToken)
	return message, nil
}

func writeWebSocketJSON(ctx context.Context, connection *websocket.Conn, payload any) error {
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := wsjson.Write(writeCtx, connection, payload); err != nil {
		return fmt.Errorf("write websocket json: %w", err)
	}

	return nil
}
