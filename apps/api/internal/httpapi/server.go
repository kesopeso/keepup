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

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
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
	StartSharing(context.Context, string, string) (routes.StartSharingResult, error)
	StopSharing(context.Context, string, string) (routes.StopSharingResult, error)
	MarkMemberOnline(context.Context, string, string) (routes.Member, bool, error)
	MarkMemberStale(context.Context, string, string) (routes.Member, bool, error)
	MarkMemberOffline(context.Context, string, string) (routes.Member, bool, error)
	RecordPosition(context.Context, string, routes.PositionUpdateInput) (routes.PositionUpdateResult, error)
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
	if authorized.Route.Status != routes.RouteStatusActive {
		_ = writeWebSocketJSON(r.Context(), connection, live.Event{
			"type":   "live_connection_rejected",
			"reason": "route_closed",
		})
		_ = connection.Close(websocket.StatusPolicyViolation, "route closed")
		return
	}
	if s.liveHub.HasMemberConnection(authorized.Route.ID, authorized.Member.ID) {
		_ = writeWebSocketJSON(r.Context(), connection, live.Event{
			"type":   "live_connection_rejected",
			"reason": "already_active_connection",
		})
		_ = connection.Close(websocket.StatusPolicyViolation, "already active")
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

	disconnectedStatus := authorized.Member.Status
	if member, changed, err := s.routes.MarkMemberOnline(r.Context(), authorized.Route.ID, authorized.Member.ID); err != nil {
		s.logger.Error("mark websocket member online failed", "error", err)
	} else if changed {
		disconnectedStatus = member.Status
		s.broadcastLiveEvent(authorized.Route.ID, live.Event{
			"type":   "member_back_online",
			"member": member,
		})
	}

	readErrCh := make(chan error, 1)
	outboundEventCh := make(chan live.Event, 16)
	trackingHealthCh := make(chan struct{}, 1)
	staleOfflineCh := make(chan struct{}, 1)
	defer func() {
		s.handleDisconnectedMember(context.Background(), authorized.Route.ID, authorized.Member.ID, disconnectedStatus)
	}()
	go func() {
		for {
			var rawMessage json.RawMessage
			var message webSocketClientMessage
			if err := wsjson.Read(r.Context(), connection, &rawMessage); err != nil {
				readErrCh <- err
				return
			}
			if err := json.Unmarshal(rawMessage, &message); err != nil {
				readErrCh <- err
				return
			}

			switch message.Type {
			case "start_sharing":
				wasTracking := disconnectedStatus == routes.MemberStatusTracking
				result, err := s.routes.StartSharing(r.Context(), authorized.Route.Code, authMessage.MemberToken)
				if err != nil {
					if !enqueueLiveEvent(r.Context(), outboundEventCh, commandRejectedEvent(message, "start_sharing", err)) {
						return
					}
					continue
				}

				if !enqueueLiveEvent(r.Context(), outboundEventCh, commandAckEvent(message, "start_sharing")) {
					return
				}
				if wasTracking {
					continue
				}
				eventType := "member_started_sharing"
				if disconnectedStatus == routes.MemberStatusStale {
					eventType = "member_back_online"
				}
				disconnectedStatus = result.Member.Status
				resetTimer(trackingHealthCh)
				s.broadcastLiveEvent(result.Member.RouteID, live.Event{
					"type":    eventType,
					"member":  result.Member,
					"segment": result.Segment,
				})
			case "stop_sharing":
				wasSpectating := disconnectedStatus == routes.MemberStatusSpectating
				result, err := s.routes.StopSharing(r.Context(), authorized.Route.Code, authMessage.MemberToken)
				if err != nil {
					if !enqueueLiveEvent(r.Context(), outboundEventCh, commandRejectedEvent(message, "stop_sharing", err)) {
						return
					}
					continue
				}

				if !enqueueLiveEvent(r.Context(), outboundEventCh, commandAckEvent(message, "stop_sharing")) {
					return
				}
				if wasSpectating {
					continue
				}
				disconnectedStatus = result.Member.Status
				s.broadcastLiveEvent(result.Member.RouteID, live.Event{
					"type":   "member_stopped_sharing",
					"member": result.Member,
				})
			case "position_update":
				input, err := positionUpdateInput(message, rawMessage)
				if err != nil {
					if !enqueueLiveEvent(r.Context(), outboundEventCh, live.Event{
						"type":  "position_rejected",
						"error": routeErrorReason(err),
					}) {
						return
					}
					continue
				}

				result, err := s.routes.RecordPosition(r.Context(), authMessage.MemberToken, input)
				if err != nil {
					if !enqueueLiveEvent(r.Context(), outboundEventCh, live.Event{
						"type":  "position_rejected",
						"error": routeErrorReason(err),
					}) {
						return
					}
					continue
				}

				if result.RecoveredMember != nil {
					disconnectedStatus = result.RecoveredMember.Status
					s.broadcastLiveEvent(result.RouteID, live.Event{
						"type":   "member_back_online",
						"member": result.RecoveredMember,
					})
				}
				resetTimer(trackingHealthCh)
				s.broadcastLiveEvent(result.RouteID, live.Event{
					"type":      "position_updated",
					"memberId":  result.MemberID,
					"segmentId": result.SegmentID,
					"point":     result.Point,
				})
			default:
				if !enqueueLiveEvent(r.Context(), outboundEventCh, live.Event{
					"type":  "message_rejected",
					"error": "unknown_message",
				}) {
					return
				}
			}
		}
	}()
	go s.runTrackingHealthTimer(r.Context(), authorized.Route.ID, authorized.Member.ID, trackingHealthCh, staleOfflineCh, &disconnectedStatus)
	if authorized.Member.Status == routes.MemberStatusTracking {
		resetTimer(trackingHealthCh)
	}
	if authorized.Member.Status == routes.MemberStatusStale {
		go s.markOfflineAfter(context.Background(), authorized.Route.ID, authorized.Member.ID, s.appConfig.TrackingOfflineAfter)
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case err := <-readErrCh:
			s.logger.Debug("websocket read loop ended", "error", err)
			return
		case event := <-outboundEventCh:
			if err := writeWebSocketJSON(r.Context(), connection, event); err != nil {
				s.logger.Debug("websocket direct event write failed", "error", err)
				return
			}
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

func (s *Server) runTrackingHealthTimer(ctx context.Context, routeID, memberID string, resetCh <-chan struct{}, _ <-chan struct{}, disconnectedStatus *string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-resetCh:
		}

		timer := time.NewTimer(s.appConfig.TrackingStaleAfter)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-resetCh:
			timer.Stop()
			continue
		case <-timer.C:
		}

		member, changed, err := s.routes.MarkMemberStale(context.Background(), routeID, memberID)
		if err != nil {
			s.logger.Error("tracking stale transition failed", "error", err)
			continue
		}
		if !changed {
			continue
		}

		*disconnectedStatus = member.Status
		s.broadcastLiveEvent(routeID, live.Event{
			"type":   "member_became_stale",
			"member": member,
		})

		offlineTimer := time.NewTimer(s.appConfig.TrackingOfflineAfter)
		select {
		case <-ctx.Done():
			offlineTimer.Stop()
			return
		case <-resetCh:
			offlineTimer.Stop()
			continue
		case <-offlineTimer.C:
		}

		offlineMember, offlineChanged, err := s.routes.MarkMemberOffline(context.Background(), routeID, memberID)
		if err != nil {
			s.logger.Error("tracking offline transition failed", "error", err)
			continue
		}
		if offlineChanged {
			*disconnectedStatus = offlineMember.Status
			s.broadcastLiveEvent(routeID, live.Event{
				"type":   "member_went_offline",
				"member": offlineMember,
			})
		}
	}
}

func (s *Server) handleDisconnectedMember(ctx context.Context, routeID, memberID, status string) {
	switch status {
	case routes.MemberStatusTracking:
		member, changed, err := s.routes.MarkMemberStale(ctx, routeID, memberID)
		if err != nil {
			s.logger.Error("disconnect stale transition failed", "error", err)
			return
		}
		if changed {
			s.broadcastLiveEvent(routeID, live.Event{
				"type":   "member_became_stale",
				"member": member,
			})
			go s.markOfflineAfter(ctx, routeID, memberID, s.appConfig.TrackingOfflineAfter)
		}
	case routes.MemberStatusSpectating:
		go s.markOfflineAfter(ctx, routeID, memberID, s.appConfig.SpectatorOfflineAfter)
	case routes.MemberStatusStale:
		go s.markOfflineAfter(ctx, routeID, memberID, s.appConfig.TrackingOfflineAfter)
	}
}

func (s *Server) markOfflineAfter(ctx context.Context, routeID, memberID string, delay time.Duration) {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}

	member, changed, err := s.routes.MarkMemberOffline(context.Background(), routeID, memberID)
	if err != nil {
		s.logger.Error("offline transition failed", "error", err)
		return
	}
	if changed {
		s.broadcastLiveEvent(routeID, live.Event{
			"type":   "member_went_offline",
			"member": member,
		})
	}
}

func resetTimer(resetCh chan<- struct{}) {
	select {
	case resetCh <- struct{}{}:
	default:
	}
}

func commandAckEvent(message webSocketClientMessage, command string) live.Event {
	return live.Event{
		"type":      "command_ack",
		"requestId": message.RequestID,
		"command":   command,
	}
}

func commandRejectedEvent(message webSocketClientMessage, command string, err error) live.Event {
	return live.Event{
		"type":      "command_rejected",
		"requestId": message.RequestID,
		"command":   command,
		"reason":    routeErrorReason(err),
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
	status, reason := routeErrorStatusAndReason(err)
	if status == http.StatusInternalServerError {
		s.logger.Error("route handler failed", "error", err)
	}
	s.writeError(w, status, reason)
}

func routeErrorReason(err error) string {
	_, reason := routeErrorStatusAndReason(err)
	return reason
}

func routeErrorStatusAndReason(err error) (int, string) {
	switch {
	case errors.Is(err, routes.ErrInvalidInput):
		return http.StatusBadRequest, "invalid_input"
	case errors.Is(err, routes.ErrRouteNotFound):
		return http.StatusNotFound, "route_not_found"
	case errors.Is(err, routes.ErrInvalidPassword):
		return http.StatusUnauthorized, "invalid_password"
	case errors.Is(err, routes.ErrAliasTaken):
		return http.StatusConflict, "alias_taken"
	case errors.Is(err, routes.ErrUnauthorized):
		return http.StatusUnauthorized, "unauthorized"
	case errors.Is(err, routes.ErrRouteClosed):
		return http.StatusConflict, "route_closed"
	case errors.Is(err, routes.ErrSharingNotAllowed):
		return http.StatusForbidden, "sharing_not_allowed"
	case errors.Is(err, routes.ErrTrackingLimitReached):
		return http.StatusConflict, "tracking_limit_reached"
	default:
		return http.StatusInternalServerError, "internal_error"
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
	defer func() {
		_ = body.Close()
	}()

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

type webSocketClientMessage struct {
	Type             string     `json:"type"`
	RequestID        string     `json:"requestId"`
	Latitude         *float64   `json:"latitude"`
	Longitude        *float64   `json:"longitude"`
	AccuracyM        *float64   `json:"accuracyM"`
	AltitudeM        *float64   `json:"altitudeM"`
	SpeedMPS         *float64   `json:"speedMps"`
	HeadingDeg       *float64   `json:"headingDeg"`
	ClientRecordedAt *time.Time `json:"clientRecordedAt"`
}

func positionUpdateInput(message webSocketClientMessage, rawPayload json.RawMessage) (routes.PositionUpdateInput, error) {
	if message.Latitude == nil || message.Longitude == nil {
		return routes.PositionUpdateInput{}, routes.ErrInvalidInput
	}

	return routes.PositionUpdateInput{
		Latitude:         *message.Latitude,
		Longitude:        *message.Longitude,
		AccuracyM:        message.AccuracyM,
		AltitudeM:        message.AltitudeM,
		SpeedMPS:         message.SpeedMPS,
		HeadingDeg:       message.HeadingDeg,
		ClientRecordedAt: message.ClientRecordedAt,
		RawPayload:       append(json.RawMessage(nil), rawPayload...),
	}, nil
}

func enqueueLiveEvent(ctx context.Context, events chan<- live.Event, event live.Event) bool {
	select {
	case events <- event:
		return true
	case <-ctx.Done():
		return false
	}
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
