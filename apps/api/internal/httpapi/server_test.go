package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keepup/apps/api/internal/config"
	"keepup/apps/api/internal/routes"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type stubHealthChecker struct {
	err error
}

func (s stubHealthChecker) Ping(_ context.Context) error {
	return s.err
}

type stubRouteService struct {
	accessRouteFn     func(context.Context, string) (routes.AccessRouteResult, error)
	authorizeMemberFn func(context.Context, string) (routes.AuthorizedMember, error)
	createRouteFn     func(context.Context, routes.CreateRouteInput) (routes.CreateRouteResult, error)
	deleteRouteFn     func(context.Context, string, string) error
	joinRouteFn       func(context.Context, string, routes.JoinRouteInput) (routes.JoinRouteResult, error)
	leaveRouteFn      func(context.Context, string, string) (routes.LeaveRouteResult, error)
	snapshotFn        func(context.Context, string, string) (routes.Snapshot, error)
	startSharingFn    func(context.Context, string, string) (routes.StartSharingResult, error)
	stopSharingFn     func(context.Context, string, string) (routes.StopSharingResult, error)
	recordPositionFn  func(context.Context, string, routes.PositionUpdateInput) (routes.PositionUpdateResult, error)
	updateRouteFn     func(context.Context, string, string, routes.UpdateRouteInput) (routes.Route, error)
}

func (s stubRouteService) CreateRoute(ctx context.Context, input routes.CreateRouteInput) (routes.CreateRouteResult, error) {
	if s.createRouteFn == nil {
		return routes.CreateRouteResult{}, nil
	}

	return s.createRouteFn(ctx, input)
}

func (s stubRouteService) AccessRoute(ctx context.Context, code string) (routes.AccessRouteResult, error) {
	if s.accessRouteFn == nil {
		return routes.AccessRouteResult{}, nil
	}

	return s.accessRouteFn(ctx, code)
}

func (s stubRouteService) AuthorizeMember(ctx context.Context, memberToken string) (routes.AuthorizedMember, error) {
	if s.authorizeMemberFn == nil {
		return routes.AuthorizedMember{}, nil
	}

	return s.authorizeMemberFn(ctx, memberToken)
}

func (s stubRouteService) JoinRoute(ctx context.Context, code string, input routes.JoinRouteInput) (routes.JoinRouteResult, error) {
	if s.joinRouteFn == nil {
		return routes.JoinRouteResult{}, nil
	}

	return s.joinRouteFn(ctx, code, input)
}

func (s stubRouteService) Snapshot(ctx context.Context, code, memberToken string) (routes.Snapshot, error) {
	if s.snapshotFn == nil {
		return routes.Snapshot{}, nil
	}

	return s.snapshotFn(ctx, code, memberToken)
}

func (s stubRouteService) StartSharing(ctx context.Context, code, memberToken string) (routes.StartSharingResult, error) {
	if s.startSharingFn == nil {
		return routes.StartSharingResult{}, nil
	}

	return s.startSharingFn(ctx, code, memberToken)
}

func (s stubRouteService) StopSharing(ctx context.Context, code, memberToken string) (routes.StopSharingResult, error) {
	if s.stopSharingFn == nil {
		return routes.StopSharingResult{}, nil
	}

	return s.stopSharingFn(ctx, code, memberToken)
}

func (s stubRouteService) RecordPosition(ctx context.Context, memberToken string, input routes.PositionUpdateInput) (routes.PositionUpdateResult, error) {
	if s.recordPositionFn == nil {
		return routes.PositionUpdateResult{}, nil
	}

	return s.recordPositionFn(ctx, memberToken, input)
}

func (s stubRouteService) UpdateRoute(ctx context.Context, code, ownerToken string, input routes.UpdateRouteInput) (routes.Route, error) {
	if s.updateRouteFn == nil {
		return routes.Route{}, nil
	}

	return s.updateRouteFn(ctx, code, ownerToken, input)
}

func (s stubRouteService) LeaveRoute(ctx context.Context, code, memberToken string) (routes.LeaveRouteResult, error) {
	if s.leaveRouteFn == nil {
		return routes.LeaveRouteResult{}, nil
	}

	return s.leaveRouteFn(ctx, code, memberToken)
}

func (s stubRouteService) DeleteRoute(ctx context.Context, code, ownerToken string) error {
	if s.deleteRouteFn == nil {
		return nil
	}

	return s.deleteRouteFn(ctx, code, ownerToken)
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
				stubRouteService{},
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
		stubRouteService{},
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

func TestCreateRouteHandler(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{
			createRouteFn: func(_ context.Context, input routes.CreateRouteInput) (routes.CreateRouteResult, error) {
				if input.Name != "Morning convoy" {
					t.Fatalf("CreateRoute() name = %q, want Morning convoy", input.Name)
				}

				return routes.CreateRouteResult{
					Route: routes.Route{
						Code:               "K7P9QD",
						Name:               input.Name,
						SharingPolicy:      input.SharingPolicy,
						Status:             routes.RouteStatusActive,
						MaxTrackingMembers: 10,
					},
					Owner: routes.Member{
						ID:          "member-1",
						DisplayName: input.DisplayName,
						IsOwner:     true,
					},
					MemberToken: "member-token",
					OwnerToken:  "owner-token",
				}, nil
			},
		},
	)

	body := map[string]string{
		"clientId":      "client-1",
		"displayName":   "Ana",
		"transportMode": "car",
		"name":          "Morning convoy",
		"sharingPolicy": routes.SharingPolicyEveryoneCanShare,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/routes", bytes.NewReader(payload))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	if !strings.Contains(recorder.Body.String(), `"memberToken":"member-token"`) {
		t.Fatalf("ServeHTTP() body = %q, want member token", recorder.Body.String())
	}
}

func TestRouteAccessHandler(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{
			accessRouteFn: func(_ context.Context, code string) (routes.AccessRouteResult, error) {
				if code != "K7P9QD" {
					t.Fatalf("AccessRoute() code = %q, want K7P9QD", code)
				}

				return routes.AccessRouteResult{
					Code:             "K7P9QD",
					Name:             "Morning convoy",
					Status:           routes.RouteStatusActive,
					RequiresPassword: true,
					SharingPolicy:    routes.SharingPolicyEveryoneCanShare,
				}, nil
			},
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/routes/K7P9QD/access", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !strings.Contains(recorder.Body.String(), `"requiresPassword":true`) {
		t.Fatalf("ServeHTTP() body = %q, want requiresPassword", recorder.Body.String())
	}
}

func TestRouteHandlerRequiresBearerToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{},
	)

	request := httptest.NewRequest(http.MethodGet, "/routes/K7P9QD", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestRouteHandler(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{
			snapshotFn: func(_ context.Context, code, token string) (routes.Snapshot, error) {
				if code != "K7P9QD" {
					t.Fatalf("Snapshot() code = %q, want K7P9QD", code)
				}

				if token != "member-token" {
					t.Fatalf("Snapshot() token = %q, want member-token", token)
				}

				return routes.Snapshot{
					Route: routes.Route{
						Code:               "K7P9QD",
						Name:               "Morning convoy",
						Status:             routes.RouteStatusActive,
						SharingPolicy:      routes.SharingPolicyEveryoneCanShare,
						MaxTrackingMembers: 10,
						CreatedAt:          now,
					},
					Members: []routes.SnapshotMember{
						{
							ID:            "member-1",
							DisplayName:   "Ana",
							TransportMode: "car",
							Role:          routes.RoleOwner,
							Status:        routes.MemberStatusSpectating,
							Color:         "#22c55e",
							JoinedAt:      now,
							Paths:         []routes.PathSegment{},
						},
					},
					Viewer: routes.ViewerCapabilities{
						MemberID:        "member-1",
						Role:            routes.RoleOwner,
						Status:          routes.MemberStatusSpectating,
						CanStartSharing: true,
						CanCloseRoute:   true,
					},
				}, nil
			},
		},
	)

	request := httptest.NewRequest(http.MethodGet, "/routes/K7P9QD", nil)
	request.Header.Set("Authorization", "Bearer member-token")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !strings.Contains(recorder.Body.String(), `"canCloseRoute":true`) {
		t.Fatalf("ServeHTTP() body = %q, want viewer permissions", recorder.Body.String())
	}
}

func TestCreateRouteMemberHandler(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{
			joinRouteFn: func(_ context.Context, code string, input routes.JoinRouteInput) (routes.JoinRouteResult, error) {
				if code != "K7P9QD" {
					t.Fatalf("JoinRoute() code = %q, want K7P9QD", code)
				}

				if input.DisplayName != "Matej" {
					t.Fatalf("JoinRoute() displayName = %q, want Matej", input.DisplayName)
				}

				return routes.JoinRouteResult{
					Route: routes.Route{
						Code:          "K7P9QD",
						Name:          "Morning convoy",
						SharingPolicy: routes.SharingPolicyEveryoneCanShare,
						Status:        routes.RouteStatusActive,
					},
					Member: routes.Member{
						ID:          "member-2",
						DisplayName: input.DisplayName,
					},
					MemberToken: "member-token",
				}, nil
			},
		},
	)

	body := map[string]string{
		"clientId":      "client-2",
		"displayName":   "Matej",
		"transportMode": "train",
		"password":      "secret",
	}

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/routes/K7P9QD/members", bytes.NewReader(payload))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	if !strings.Contains(recorder.Body.String(), `"memberToken":"member-token"`) {
		t.Fatalf("ServeHTTP() body = %q, want member token", recorder.Body.String())
	}
}

func TestUpdateRouteHandler(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{
			updateRouteFn: func(_ context.Context, code, token string, input routes.UpdateRouteInput) (routes.Route, error) {
				if code != "K7P9QD" || token != "owner-token" {
					t.Fatalf("UpdateRoute() got code=%q token=%q", code, token)
				}

				if input.Status != routes.RouteStatusClosed {
					t.Fatalf("UpdateRoute() status = %q, want closed", input.Status)
				}

				return routes.Route{
					Code:          code,
					Name:          "Morning convoy",
					Status:        routes.RouteStatusClosed,
					SharingPolicy: routes.SharingPolicyEveryoneCanShare,
				}, nil
			},
		},
	)

	body := map[string]string{
		"status": routes.RouteStatusClosed,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPatch, "/routes/K7P9QD", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer owner-token")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !strings.Contains(recorder.Body.String(), `"status":"closed"`) {
		t.Fatalf("ServeHTTP() body = %q, want closed status", recorder.Body.String())
	}
}

func TestLeaveRouteHandler(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{
			leaveRouteFn: func(_ context.Context, code, token string) (routes.LeaveRouteResult, error) {
				if code != "K7P9QD" || token != "member-token" {
					t.Fatalf("LeaveRoute() got code=%q token=%q", code, token)
				}

				now := time.Now().UTC()
				return routes.LeaveRouteResult{
					Member: routes.Member{
						ID:     "member-2",
						Status: routes.MemberStatusLeft,
						LeftAt: &now,
					},
				}, nil
			},
		},
	)

	request := httptest.NewRequest(http.MethodDelete, "/routes/K7P9QD/members/me", nil)
	request.Header.Set("Authorization", "Bearer member-token")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !strings.Contains(recorder.Body.String(), `"status":"left"`) {
		t.Fatalf("ServeHTTP() body = %q, want left status", recorder.Body.String())
	}
}

func TestMemberSharingHandlerStartsSharing(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{
			startSharingFn: func(_ context.Context, code, token string) (routes.StartSharingResult, error) {
				if code != "K7P9QD" || token != "member-token" {
					t.Fatalf("StartSharing() got code=%q token=%q", code, token)
				}

				return routes.StartSharingResult{
					Member: routes.Member{
						ID:      "member-2",
						RouteID: "route-1",
						Status:  routes.MemberStatusTracking,
					},
					Segment: routes.PathSegment{
						ID:        "segment-1",
						StartedAt: &now,
						Points:    []routes.RoutePoint{},
					},
				}, nil
			},
		},
	)

	request := httptest.NewRequest(http.MethodPut, "/routes/K7P9QD/members/me/sharing", strings.NewReader(`{"enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer member-token")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !strings.Contains(recorder.Body.String(), `"status":"tracking"`) {
		t.Fatalf("ServeHTTP() body = %q, want tracking status", recorder.Body.String())
	}
}

func TestMemberSharingHandlerStopsSharing(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{
			stopSharingFn: func(_ context.Context, code, token string) (routes.StopSharingResult, error) {
				if code != "K7P9QD" || token != "member-token" {
					t.Fatalf("StopSharing() got code=%q token=%q", code, token)
				}

				return routes.StopSharingResult{
					Member: routes.Member{
						ID:      "member-2",
						RouteID: "route-1",
						Status:  routes.MemberStatusSpectating,
					},
				}, nil
			},
		},
	)

	request := httptest.NewRequest(http.MethodPut, "/routes/K7P9QD/members/me/sharing", strings.NewReader(`{"enabled":false}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer member-token")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !strings.Contains(recorder.Body.String(), `"status":"spectating"`) {
		t.Fatalf("ServeHTTP() body = %q, want spectating status", recorder.Body.String())
	}
}

func TestDeleteRouteHandler(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080"},
		stubHealthChecker{},
		stubRouteService{
			deleteRouteFn: func(_ context.Context, code, token string) error {
				if code != "K7P9QD" || token != "owner-token" {
					t.Fatalf("DeleteRoute() got code=%q token=%q", code, token)
				}

				return nil
			},
		},
	)

	request := httptest.NewRequest(http.MethodDelete, "/routes/K7P9QD", nil)
	request.Header.Set("Authorization", "Bearer owner-token")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("ServeHTTP() status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}

func TestWebSocketAuthenticatesFirstMessage(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080", WebSocketAuthTimeout: time.Second},
		stubHealthChecker{},
		stubRouteService{
			authorizeMemberFn: func(_ context.Context, token string) (routes.AuthorizedMember, error) {
				if token != "member-token" {
					t.Fatalf("AuthorizeMember() token = %q, want member-token", token)
				}

				return routes.AuthorizedMember{
					Route: routes.Route{
						ID:     "route-1",
						Code:   "K7P9QD",
						Status: routes.RouteStatusActive,
					},
					Member: routes.Member{
						ID:     "member-1",
						Status: routes.MemberStatusSpectating,
					},
				}, nil
			},
		},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	connection, _, err := websocket.Dial(ctx, webSocketURL(server.URL), nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer func() {
		_ = connection.Close(websocket.StatusNormalClosure, "test complete")
	}()

	if err := wsjson.Write(ctx, connection, map[string]string{
		"type":        "authenticate",
		"memberToken": "member-token",
	}); err != nil {
		t.Fatalf("wsjson.Write() error = %v", err)
	}

	var event struct {
		Type  string `json:"type"`
		Route struct {
			Code string `json:"code"`
		} `json:"route"`
		Member struct {
			ID string `json:"id"`
		} `json:"member"`
	}
	if err := wsjson.Read(ctx, connection, &event); err != nil {
		t.Fatalf("wsjson.Read() error = %v", err)
	}

	if event.Type != "connection_established" {
		t.Fatalf("event type = %q, want connection_established", event.Type)
	}

	if event.Route.Code != "K7P9QD" || event.Member.ID != "member-1" {
		t.Fatalf("event route/member = %q/%q, want K7P9QD/member-1", event.Route.Code, event.Member.ID)
	}
}

func TestWebSocketReceivesMemberJoinedBroadcast(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080", WebSocketAuthTimeout: time.Second},
		stubHealthChecker{},
		stubRouteService{
			authorizeMemberFn: func(_ context.Context, token string) (routes.AuthorizedMember, error) {
				if token != "member-token" {
					t.Fatalf("AuthorizeMember() token = %q, want member-token", token)
				}

				return routes.AuthorizedMember{
					Route: routes.Route{
						ID:     "route-1",
						Code:   "K7P9QD",
						Status: routes.RouteStatusActive,
					},
					Member: routes.Member{
						ID:     "member-1",
						Status: routes.MemberStatusSpectating,
					},
				}, nil
			},
			joinRouteFn: func(_ context.Context, code string, input routes.JoinRouteInput) (routes.JoinRouteResult, error) {
				if code != "K7P9QD" {
					t.Fatalf("JoinRoute() code = %q, want K7P9QD", code)
				}

				return routes.JoinRouteResult{
					Route: routes.Route{
						ID:     "route-1",
						Code:   "K7P9QD",
						Status: routes.RouteStatusActive,
					},
					Member: routes.Member{
						ID:            "member-2",
						RouteID:       "route-1",
						DisplayName:   input.DisplayName,
						TransportMode: input.TransportMode,
						Status:        routes.MemberStatusSpectating,
						JoinedAt:      now,
					},
					MemberToken: "new-member-token",
				}, nil
			},
		},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	connection, _, err := websocket.Dial(ctx, webSocketURL(server.URL), nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer func() {
		_ = connection.Close(websocket.StatusNormalClosure, "test complete")
	}()

	if err := wsjson.Write(ctx, connection, map[string]string{
		"type":        "authenticate",
		"memberToken": "member-token",
	}); err != nil {
		t.Fatalf("wsjson.Write() error = %v", err)
	}

	var established map[string]any
	if err := wsjson.Read(ctx, connection, &established); err != nil {
		t.Fatalf("read connection_established error = %v", err)
	}

	body := map[string]string{
		"clientId":      "client-2",
		"displayName":   "Matej",
		"transportMode": "train",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	response, err := http.Post(server.URL+"/routes/K7P9QD/members", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("http.Post() error = %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("POST /routes/K7P9QD/members status = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	var event struct {
		Type   string `json:"type"`
		Member struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
			Status      string `json:"status"`
		} `json:"member"`
	}
	if err := wsjson.Read(ctx, connection, &event); err != nil {
		t.Fatalf("read member_joined event error = %v", err)
	}

	if event.Type != "member_joined" {
		t.Fatalf("event type = %q, want member_joined", event.Type)
	}

	if event.Member.ID != "member-2" || event.Member.DisplayName != "Matej" || event.Member.Status != routes.MemberStatusSpectating {
		t.Fatalf("event member = %#v, want joined member", event.Member)
	}
}

func TestWebSocketRecordsAndBroadcastsPositionUpdate(t *testing.T) {
	t.Parallel()

	recordedAt := time.Now().UTC()
	clientRecordedAt := recordedAt.Add(-2 * time.Second).UTC()
	accuracy := 8.5
	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080", WebSocketAuthTimeout: time.Second},
		stubHealthChecker{},
		stubRouteService{
			authorizeMemberFn: func(_ context.Context, token string) (routes.AuthorizedMember, error) {
				if token != "member-token" {
					t.Fatalf("AuthorizeMember() token = %q, want member-token", token)
				}

				return routes.AuthorizedMember{
					Route: routes.Route{
						ID:     "route-1",
						Code:   "K7P9QD",
						Status: routes.RouteStatusActive,
					},
					Member: routes.Member{
						ID:     "member-1",
						Status: routes.MemberStatusTracking,
					},
				}, nil
			},
			recordPositionFn: func(_ context.Context, token string, input routes.PositionUpdateInput) (routes.PositionUpdateResult, error) {
				if token != "member-token" {
					t.Fatalf("RecordPosition() token = %q, want member-token", token)
				}

				if input.Latitude != 46.0569 || input.Longitude != 14.5058 {
					t.Fatalf("RecordPosition() coordinates = %f,%f", input.Latitude, input.Longitude)
				}

				if input.AccuracyM == nil || *input.AccuracyM != accuracy {
					t.Fatal("RecordPosition() expected accuracy")
				}

				if input.ClientRecordedAt == nil || !input.ClientRecordedAt.Equal(clientRecordedAt) {
					t.Fatal("RecordPosition() expected client recorded time")
				}

				if !strings.Contains(string(input.RawPayload), `"type":"position_update"`) {
					t.Fatalf("RecordPosition() raw payload = %s, want original message", string(input.RawPayload))
				}

				return routes.PositionUpdateResult{
					RouteID:   "route-1",
					MemberID:  "member-1",
					SegmentID: "segment-1",
					Point: routes.RoutePoint{
						Seq:              7,
						Latitude:         input.Latitude,
						Longitude:        input.Longitude,
						AccuracyM:        input.AccuracyM,
						ClientRecordedAt: input.ClientRecordedAt,
						RecordedAt:       recordedAt,
					},
				}, nil
			},
		},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	connection, _, err := websocket.Dial(ctx, webSocketURL(server.URL), nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer func() {
		_ = connection.Close(websocket.StatusNormalClosure, "test complete")
	}()

	if err := wsjson.Write(ctx, connection, map[string]string{
		"type":        "authenticate",
		"memberToken": "member-token",
	}); err != nil {
		t.Fatalf("write authenticate error = %v", err)
	}

	var established map[string]any
	if err := wsjson.Read(ctx, connection, &established); err != nil {
		t.Fatalf("read connection_established error = %v", err)
	}

	if err := wsjson.Write(ctx, connection, map[string]any{
		"type":             "position_update",
		"latitude":         46.0569,
		"longitude":        14.5058,
		"accuracyM":        accuracy,
		"clientRecordedAt": clientRecordedAt,
	}); err != nil {
		t.Fatalf("write position_update error = %v", err)
	}

	var event struct {
		Type      string `json:"type"`
		MemberID  string `json:"memberId"`
		SegmentID string `json:"segmentId"`
		Point     struct {
			Seq       int64   `json:"seq"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"point"`
	}
	if err := wsjson.Read(ctx, connection, &event); err != nil {
		t.Fatalf("read position_updated event error = %v", err)
	}

	if event.Type != "position_updated" {
		t.Fatalf("event type = %q, want position_updated", event.Type)
	}

	if event.MemberID != "member-1" || event.SegmentID != "segment-1" {
		t.Fatalf("event member/segment = %q/%q, want member-1/segment-1", event.MemberID, event.SegmentID)
	}

	if event.Point.Seq != 7 || event.Point.Latitude != 46.0569 || event.Point.Longitude != 14.5058 {
		t.Fatalf("event point = %#v, want accepted point", event.Point)
	}
}

func TestWebSocketRejectsInvalidPositionUpdate(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080", WebSocketAuthTimeout: time.Second},
		stubHealthChecker{},
		stubRouteService{
			authorizeMemberFn: func(_ context.Context, _ string) (routes.AuthorizedMember, error) {
				return routes.AuthorizedMember{
					Route: routes.Route{
						ID:     "route-1",
						Code:   "K7P9QD",
						Status: routes.RouteStatusActive,
					},
					Member: routes.Member{
						ID:     "member-1",
						Status: routes.MemberStatusTracking,
					},
				}, nil
			},
			recordPositionFn: func(context.Context, string, routes.PositionUpdateInput) (routes.PositionUpdateResult, error) {
				t.Fatal("RecordPosition() should not run for a malformed position message")
				return routes.PositionUpdateResult{}, nil
			},
		},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	connection, _, err := websocket.Dial(ctx, webSocketURL(server.URL), nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer func() {
		_ = connection.Close(websocket.StatusNormalClosure, "test complete")
	}()

	if err := wsjson.Write(ctx, connection, map[string]string{
		"type":        "authenticate",
		"memberToken": "member-token",
	}); err != nil {
		t.Fatalf("write authenticate error = %v", err)
	}

	var established map[string]any
	if err := wsjson.Read(ctx, connection, &established); err != nil {
		t.Fatalf("read connection_established error = %v", err)
	}

	if err := wsjson.Write(ctx, connection, map[string]any{
		"type":      "position_update",
		"longitude": 14.5058,
	}); err != nil {
		t.Fatalf("write position_update error = %v", err)
	}

	var event struct {
		Type  string `json:"type"`
		Error string `json:"error"`
	}
	if err := wsjson.Read(ctx, connection, &event); err != nil {
		t.Fatalf("read position_rejected event error = %v", err)
	}

	if event.Type != "position_rejected" || event.Error != "invalid_input" {
		t.Fatalf("event = %#v, want invalid position rejection", event)
	}
}

func TestWebSocketRejectsInvalidFirstMessageToken(t *testing.T) {
	t.Parallel()

	handler := NewHandler(
		slog.New(slog.NewTextHandler(testWriter{t: t}, nil)),
		config.AppConfig{Env: "test", Port: "8080", WebSocketAuthTimeout: time.Second},
		stubHealthChecker{},
		stubRouteService{
			authorizeMemberFn: func(_ context.Context, _ string) (routes.AuthorizedMember, error) {
				return routes.AuthorizedMember{}, routes.ErrUnauthorized
			},
		},
	)
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	connection, _, err := websocket.Dial(ctx, webSocketURL(server.URL), nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer func() {
		_ = connection.Close(websocket.StatusNormalClosure, "test complete")
	}()

	if err := wsjson.Write(ctx, connection, map[string]string{
		"type":        "authenticate",
		"memberToken": "bad-token",
	}); err != nil {
		t.Fatalf("wsjson.Write() error = %v", err)
	}

	var event map[string]any
	err = wsjson.Read(ctx, connection, &event)
	if err == nil {
		t.Fatal("wsjson.Read() error = nil, want close error")
	}

	if status := websocket.CloseStatus(err); status != websocket.StatusPolicyViolation {
		t.Fatalf("websocket close status = %v, want %v", status, websocket.StatusPolicyViolation)
	}
}

func webSocketURL(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/ws"
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}
