package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"keepup/apps/api/internal/config"
	"keepup/apps/api/internal/live"
	"keepup/apps/api/internal/routes"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func TestStaleReconnectTimerEndsWhenSharingChanges(t *testing.T) {
	for _, command := range []string{"start_sharing", "stop_sharing", "position_update"} {
		t.Run(command, func(t *testing.T) {
			offline := make(chan struct{}, 4)
			member := routes.Member{ID: "member", RouteID: "route", Status: routes.MemberStatusStale}
			cfg := config.AppConfig{TrackingOfflineAfter: 100 * time.Millisecond, TrackingStaleAfter: time.Hour, SpectatorOfflineAfter: time.Hour}
			handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, stubHealthChecker{}, stubRouteService{
				authorizeMemberFn: func(context.Context, string) (routes.AuthorizedMember, error) {
					return routes.AuthorizedMember{Route: routes.Route{ID: "route", Code: "ABC123", Status: routes.RouteStatusActive}, Member: member}, nil
				},
				startSharingFn: func(context.Context, string, string) (routes.StartSharingResult, error) {
					m := member
					m.Status = routes.MemberStatusTracking
					return routes.StartSharingResult{Member: m}, nil
				},
				stopSharingFn: func(context.Context, string, string) (routes.StopSharingResult, error) {
					m := member
					m.Status = routes.MemberStatusSpectating
					return routes.StopSharingResult{Member: m}, nil
				},
				recordPositionFn: func(context.Context, string, routes.PositionUpdateInput) (routes.PositionUpdateResult, error) {
					m := member
					m.Status = routes.MemberStatusTracking
					return routes.PositionUpdateResult{RouteID: "route", MemberID: "member", RecoveredMember: &m}, nil
				},
				markOfflineFn: func(context.Context, string, string) (routes.Member, bool, error) {
					offline <- struct{}{}
					return member, false, nil
				},
			})
			server := httptest.NewServer(handler)
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			connection := dialAuthenticatedWebSocket(t, ctx, server.URL, "token")
			defer func() { _ = connection.Close(websocket.StatusNormalClosure, "done") }()
			if err := wsjson.Write(ctx, connection, map[string]any{"type": command, "requestId": "change", "latitude": 46, "longitude": 14, "accuracyM": 5, "clientRecordedAt": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
				t.Fatal(err)
			}
			var event struct {
				Type string `json:"type"`
			}
			expected := "command_ack"
			if command == "position_update" {
				expected = "position_updated"
			}
			for event.Type != expected {
				if err := wsjson.Read(ctx, connection, &event); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case <-offline:
				t.Fatal("obsolete stale timer marked the recovered member offline")
			case <-time.After(2 * cfg.TrackingOfflineAfter):
			}
		})
	}
}

func TestOldDisconnectionCannotExpireDuringALaterOutage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		server := &Server{liveHub: live.NewHub(), logger: slog.New(slog.NewTextHandler(io.Discard, nil)), routes: stubRouteService{
			markOfflineFn: func(context.Context, string, string) (routes.Member, bool, error) {
				calls++
				return routes.Member{}, false, nil
			},
		}}
		server.scheduleDisconnectedMemberOffline(context.Background(), "route", "member", 100*time.Second)
		synctest.Wait()
		<-time.After(10 * time.Second)
		server.cancelDisconnectedTimer("route", "member")
		server.scheduleDisconnectedMemberOffline(context.Background(), "route", "member", 100*time.Second)
		synctest.Wait()
		<-time.After(90 * time.Second)
		synctest.Wait()
		if calls != 0 {
			t.Fatal("old disconnect timer expired during replacement outage")
		}
		<-time.After(10 * time.Second)
		synctest.Wait()
		if calls != 1 {
			t.Fatalf("offline calls=%d, want 1 after the new grace period", calls)
		}
		entries := 0
		server.disconnectedTimers.Range(func(_, _ any) bool { entries++; return true })
		if entries != 0 {
			t.Fatal("expired timer retained its member entry")
		}
	})
}
