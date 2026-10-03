package httpapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"keepup/apps/api/internal/config"
	"keepup/apps/api/internal/routes"
)

func TestGPSRejectionsKeepLiveConnection(t *testing.T) {
	codes := []string{"accuracy_required", "accuracy_too_low", "timestamp_required", "timestamp_too_old", "timestamp_in_future", "duplicate_timestamp", "out_of_order_timestamp", "impossible_jump"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(logger, config.AppConfig{}, stubHealthChecker{}, stubRouteService{
		authorizeMemberFn: func(context.Context, string) (routes.AuthorizedMember, error) {
			return routes.AuthorizedMember{Route: routes.Route{ID: "r", Status: routes.RouteStatusActive}, Member: routes.Member{ID: "m", Status: routes.MemberStatusStale}}, nil
		},
		recordPositionFn: func(_ context.Context, _ string, p routes.PositionUpdateInput) (routes.PositionUpdateResult, error) {
			if int(p.Latitude) < len(codes) {
				return routes.PositionUpdateResult{}, fmt.Errorf("wrapped: %w", &routes.PositionValidationError{Code: codes[int(p.Latitude)]})
			}
			return routes.PositionUpdateResult{RouteID: "r", MemberID: "m", SegmentID: "s", Point: routes.RoutePoint{Seq: 1}, RecoveredMember: &routes.Member{ID: "m", Status: routes.MemberStatusTracking}}, nil
		},
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, webSocketURL(server.URL), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()
	if err = wsjson.Write(ctx, conn, map[string]string{"type": "authenticate", "memberToken": "token"}); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type  string `json:"type"`
		Error string `json:"error"`
	}
	if err = wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "connection_established" {
		t.Fatal(event)
	}
	for _, stamp := range []string{"bad", "2026-10-03T12:00:00"} {
		if err = wsjson.Write(ctx, conn, map[string]any{"type": "position_update", "latitude": 0, "longitude": 0, "clientRecordedAt": stamp}); err != nil {
			t.Fatal(err)
		}
		if err = wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type != "position_rejected" || event.Error != "invalid_input" {
			t.Fatal(event)
		}
	}

	for i, code := range codes {
		if err = wsjson.Write(ctx, conn, map[string]any{"type": "position_update", "latitude": i, "longitude": 0}); err != nil {
			t.Fatal(err)
		}
		if err = wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type != "position_rejected" || event.Error != code {
			t.Fatalf("got %+v want %s", event, code)
		}
	}
	if err = wsjson.Write(ctx, conn, map[string]any{"type": "position_update", "latitude": len(codes), "longitude": 0}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"member_back_online", "position_updated"} {
		if err = wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type != expected {
			t.Fatalf("got %s want %s", event.Type, expected)
		}
	}
}

func TestPositionRejectionLogsExcludeSampleData(t *testing.T) {
	var output strings.Builder
	server := Server{logger: slog.New(slog.NewTextHandler(&output, nil))}
	server.logPositionRejection(&routes.PositionValidationError{Code: "impossible_jump"})
	text := output.String()
	if !strings.Contains(text, "reason=impossible_jump") || !strings.Contains(text, "rejection_count=") {
		t.Fatal(text)
	}
	if strings.Contains(text, "latitude") || strings.Contains(text, "token") {
		t.Fatal("sensitive sample data logged")
	}
}

func TestRejectedSamplesDoNotRefreshTrackingTimer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(logger, config.AppConfig{TrackingStaleAfter: 50 * time.Millisecond, TrackingOfflineAfter: time.Hour}, stubHealthChecker{}, stubRouteService{
		authorizeMemberFn: func(context.Context, string) (routes.AuthorizedMember, error) {
			return routes.AuthorizedMember{Route: routes.Route{ID: "r", Status: routes.RouteStatusActive}, Member: routes.Member{ID: "m", Status: routes.MemberStatusTracking}}, nil
		},
		recordPositionFn: func(context.Context, string, routes.PositionUpdateInput) (routes.PositionUpdateResult, error) {
			return routes.PositionUpdateResult{}, &routes.PositionValidationError{Code: "accuracy_too_low"}
		},
		markStaleFn: func(context.Context, string, string) (routes.Member, bool, error) {
			return routes.Member{ID: "m", RouteID: "r", Status: routes.MemberStatusStale}, true, nil
		},
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, webSocketURL(server.URL), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()
	if err = wsjson.Write(ctx, conn, map[string]string{"type": "authenticate", "memberToken": "token"}); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type string `json:"type"`
	}
	if err = wsjson.Read(ctx, conn, &event); err != nil {
		t.Fatal(err)
	}
	// Continuously rejected samples must not postpone the initial stale deadline.
	for {
		if err = wsjson.Write(ctx, conn, map[string]any{"type": "position_update", "latitude": 0, "longitude": 0}); err != nil {
			t.Fatal(err)
		}
		if err = wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatal(err)
		}
		if event.Type == "member_became_stale" {
			break
		}
		if event.Type != "position_rejected" {
			t.Fatalf("unexpected event %s", event.Type)
		}
	}
}
