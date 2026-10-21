package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5"
	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/routes"
)

type accountTestSocket struct {
	conn   *websocket.Conn
	events chan map[string]any
	closed chan error
}

func openTestAccountSocket(t *testing.T, f accountHTTPFixture, serverURL string, session auth.Session, code string) *accountTestSocket {
	t.Helper()
	ctx := context.Background()
	ticket, err := f.routes.IssueSocketTicket(ctx, session, code)
	if err != nil {
		t.Fatal(err)
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(serverURL, "http")+"/ws", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://keepup.example"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	if err = wsjson.Write(dialCtx, conn, map[string]any{"type": "authenticate", "ticket": ticket.Ticket}); err != nil {
		t.Fatal(err)
	}
	socket := &accountTestSocket{conn: conn, events: make(chan map[string]any, 256), closed: make(chan error, 1)}
	readCtx, stop := context.WithCancel(ctx)
	t.Cleanup(stop)
	go func() {
		for {
			var event map[string]any
			if err := wsjson.Read(readCtx, conn, &event); err != nil {
				socket.closed <- err
				return
			}
			select {
			case socket.events <- event:
			case <-readCtx.Done():
				return
			}
		}
	}()
	return socket
}

func (s *accountTestSocket) event(t *testing.T, kind, requestID string) map[string]any {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case event := <-s.events:
			if _, exists := event["sharingGeneration"]; exists && (event["type"] != "command_ack" || event["command"] != "start_sharing") {
				t.Fatal("generation leaked", event)
			}
			if event["type"] == kind && (requestID == "" || event["requestId"] == requestID) {
				return event
			}
		case err := <-s.closed:
			t.Fatal("socket closed before event", kind, err)
		case <-timeout.C:
			t.Fatal("missing event", kind, requestID)
		}
	}
}

func (s *accountTestSocket) send(t *testing.T, message map[string]any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, s.conn, message); err != nil {
		t.Fatal(err)
	}
}

func (s *accountTestSocket) policyClosed(t *testing.T) {
	t.Helper()
	select {
	case err := <-s.closed:
		if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revoked socket remained connected")
	}
}

func TestAccountSocketMultipleViewersAndGeneration(t *testing.T) {
	f := accountHTTPTest(t)
	ctx := context.Background()
	owner := f.user(t, "MultiViewer", true)
	route, err := f.routes.CreateRoute(ctx, owner.Session, routes.CreateRouteInput{Name: "Multiple viewers", TransportMode: "walking", SharingPolicy: routes.SharingPolicyEveryoneCanShare})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.handler)
	defer server.Close()
	a := openTestAccountSocket(t, f, server.URL, owner.Session, route.Route.Code)
	first := a.event(t, "connection_established", "")
	b := openTestAccountSocket(t, f, server.URL, owner.Session, route.Route.Code)
	second := b.event(t, "connection_established", "")
	if first["connectionId"] == second["connectionId"] || first["viewer"] == nil {
		t.Fatal(first, second)
	}
	a.send(t, map[string]any{"type": "start_sharing", "requestId": "a-start"})
	ack := a.event(t, "command_ack", "a-start")
	generation, ok := ack["sharingGeneration"].(string)
	if !ok || generation == "" {
		t.Fatal(ack)
	}
	b.event(t, "member_started_sharing", "")
	caps := b.event(t, "viewer_updated", "")["viewer"].(map[string]any)
	if caps["canStartSharing"] != false || caps["canStopSharing"] != false {
		t.Fatal(caps)
	}
	b.send(t, map[string]any{"type": "start_sharing", "requestId": "b-start"})
	if result := b.event(t, "command_rejected", "b-start"); result["reason"] != "sharing_on_another_device" {
		t.Fatal(result)
	}
	b.send(t, map[string]any{"type": "stop_sharing", "requestId": "b-stop", "sharingGeneration": generation})
	if result := b.event(t, "command_rejected", "b-stop"); result["reason"] != "sharing_expired" {
		t.Fatal(result)
	}
	point := map[string]any{"type": "position_update", "latitude": 46, "longitude": 14, "accuracyM": 5, "clientRecordedAt": time.Now().UTC()}
	a.send(t, point)
	if result := a.event(t, "position_rejected", ""); result["error"] != "sharing_expired" {
		t.Fatal(result)
	}
	point["sharingGeneration"] = generation
	a.send(t, point)
	b.event(t, "position_updated", "")
	a.send(t, map[string]any{"type": "stop_sharing", "requestId": "a-stop", "sharingGeneration": generation})
	a.event(t, "command_ack", "a-stop")
	b.event(t, "member_stopped_sharing", "")
	b.send(t, map[string]any{"type": "start_sharing", "requestId": "b-now"})
	next := b.event(t, "command_ack", "b-now")
	if next["sharingGeneration"] == generation {
		t.Fatal("generation reused")
	}
	a.send(t, point)
	if result := a.event(t, "position_rejected", ""); result["error"] != "sharing_expired" {
		t.Fatal(result)
	}
	snapshot, err := f.routes.Snapshot(ctx, owner.Session, route.Route.Code)
	if err != nil || len(snapshot.Members[0].Paths) != 2 || len(snapshot.Members[0].Paths[0].Points) != 1 {
		t.Fatal(snapshot, err)
	}
}

func TestAccountLiveSynchronousRevocation(t *testing.T) {
	for _, operation := range []string{"logout", "logout_all", "password_reset", "password_changed"} {
		t.Run(operation, func(t *testing.T) {
			f := accountHTTPTest(t)
			ctx := context.Background()
			owner := f.user(t, "RevocationOwner", true)
			other, err := f.accounts.Login(ctx, "RevocationOwner", "password1!", "test")
			if err != nil {
				t.Fatal(err)
			}
			route, err := f.routes.CreateRoute(ctx, owner.Session, routes.CreateRouteInput{Name: "Revoke", TransportMode: "walking", SharingPolicy: routes.SharingPolicyEveryoneCanShare})
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(f.handler)
			defer server.Close()
			sender := openTestAccountSocket(t, f, server.URL, other.Session, route.Route.Code)
			sender.event(t, "connection_established", "")
			viewer := openTestAccountSocket(t, f, server.URL, owner.Session, route.Route.Code)
			viewer.event(t, "connection_established", "")
			sender.send(t, map[string]any{"type": "start_sharing", "requestId": "start"})
			sender.event(t, "command_ack", "start")
			switch operation {
			case "logout":
				err = f.accounts.Logout(ctx, other.Token, false)
			case "logout_all":
				err = f.accounts.Logout(ctx, owner.Token, true)
			case "password_changed":
				_, err = f.accounts.ChangePassword(ctx, owner.Token, "password1!", "changed-password1!", "test")
			case "password_reset":
				raw := strings.Repeat("r", 43)
				digest := sha256.Sum256([]byte(raw))
				_, err = f.pool.Exec(ctx, `INSERT INTO auth_tokens(user_id,purpose,token_hash,expires_at) VALUES($1,'reset',$2,now()+interval '1 hour')`, owner.Session.User.ID, hex.EncodeToString(digest[:]))
				if err == nil {
					err = f.accounts.ResetPassword(ctx, raw, "reset-password1!", "test")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			// No polling or sleeps: committed cleanup is visible as soon as the auth operation returns.
			var leases, segments, connections int
			if err = f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM account_sharing_leases),(SELECT count(*) FROM path_segments WHERE ended_at IS NULL),(SELECT count(*) FROM account_live_connections WHERE session_id=$1)`, other.Session.ID).Scan(&leases, &segments, &connections); err != nil || leases != 0 || segments != 0 || connections != 0 {
				t.Fatal(leases, segments, connections, err)
			}
			// The reader may already have consumed close; drain buffered events independently.
			var advisory map[string]any
			for len(sender.events) > 0 {
				event := <-sender.events
				if event["type"] == "session_revoked" {
					advisory = event
				}
			}
			if advisory == nil || advisory["reason"] != operation {
				t.Fatal("revocation advisory", advisory)
			}
			sender.policyClosed(t)
			if operation == "logout" || operation == "password_changed" {
				viewer.send(t, map[string]any{"type": "start_sharing", "requestId": "survivor"})
				viewer.event(t, "command_ack", "survivor")
			} else {
				viewer.policyClosed(t)
			}
		})
	}
}

type failingLiveRevoker struct {
	service  *routes.AccountService
	notified bool
}

func (h *failingLiveRevoker) Revoke(ctx context.Context, tx pgx.Tx, change *auth.Revocation) error {
	if err := h.service.Revoke(ctx, tx, change); err != nil {
		return err
	}
	return errors.New("test cleanup failure")
}
func (h *failingLiveRevoker) Revoked(auth.Revocation) { h.notified = true }

func TestAccountRevocationCleanupRollsBackWithSession(t *testing.T) {
	f := accountHTTPTest(t)
	ctx := context.Background()
	owner := f.user(t, "AtomicRevoke", true)
	route, err := f.routes.CreateRoute(ctx, owner.Session, routes.CreateRouteInput{Name: "Atomic", TransportMode: "walking", SharingPolicy: routes.SharingPolicyEveryoneCanShare})
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := f.routes.IssueSocketTicket(ctx, owner.Session, route.Route.Code)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := f.routes.ConsumeSocketTicket(ctx, ticket.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	bound, err = f.routes.OpenLive(ctx, bound)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.routes.StartSharing(ctx, bound); err != nil {
		t.Fatal(err)
	}
	hook := &failingLiveRevoker{service: f.routes}
	recovery := auth.RecoveryConfig{PublicURL: "https://keepup.example", EncryptionKey: bytes.Repeat([]byte{1}, 32), Policy: auth.DefaultRecoveryPolicy()}
	accounts, err := auth.NewService(auth.NewPostgresRepository(f.pool), auth.DefaultPolicy(), recovery, auth.WithSessionRevoker(hook))
	if err != nil {
		t.Fatal(err)
	}
	if err = accounts.Logout(ctx, owner.Token, false); err == nil {
		t.Fatal("cleanup failure ignored")
	}
	if hook.notified {
		t.Fatal("rollback sent revocation")
	}
	if _, err = accounts.Authenticate(ctx, owner.Token); err != nil {
		t.Fatal("session did not roll back", err)
	}
	var leases int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM account_sharing_leases`).Scan(&leases); err != nil || leases != 1 {
		t.Fatal("lease did not roll back", leases, err)
	}
}

type interleavedLiveRevoker struct {
	auth.SessionRevoker
	beforeNotify func()
}

func (h interleavedLiveRevoker) Revoked(change auth.Revocation) {
	h.beforeNotify()
	h.SessionRevoker.Revoked(change)
}

func TestAccountLogoutAllDoesNotCloseFreshLoginAfterCommit(t *testing.T) {
	f := accountHTTPTest(t)
	ctx := context.Background()
	owner := f.user(t, "LoginAfterLogout", true)
	route, err := f.routes.CreateRoute(ctx, owner.Session, routes.CreateRouteInput{Name: "Fresh login", TransportMode: "walking", SharingPolicy: routes.SharingPolicyEveryoneCanShare})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.handler)
	defer server.Close()
	old := openTestAccountSocket(t, f, server.URL, owner.Session, route.Route.Code)
	old.event(t, "connection_established", "")
	var fresh *accountTestSocket
	hook := interleavedLiveRevoker{SessionRevoker: f.routes, beforeNotify: func() {
		login, err := f.accounts.Login(ctx, "LoginAfterLogout", "password1!", "test")
		if err != nil {
			t.Fatal(err)
		}
		fresh = openTestAccountSocket(t, f, server.URL, login.Session, route.Route.Code)
		fresh.event(t, "connection_established", "")
	}}
	recovery := auth.RecoveryConfig{PublicURL: "https://keepup.example", EncryptionKey: bytes.Repeat([]byte{1}, 32), Policy: auth.DefaultRecoveryPolicy()}
	accounts, err := auth.NewService(auth.NewPostgresRepository(f.pool), auth.DefaultPolicy(), recovery, auth.WithSessionRevoker(hook))
	if err != nil {
		t.Fatal(err)
	}
	if err = accounts.Logout(ctx, owner.Token, true); err != nil {
		t.Fatal(err)
	}
	old.policyClosed(t)
	fresh.send(t, map[string]any{"type": "start_sharing", "requestId": "fresh"})
	fresh.event(t, "command_ack", "fresh")
}

func TestAccountSocketLifecycleAccessLoss(t *testing.T) {
	for _, action := range []string{"leave", "close", "delete"} {
		t.Run(action, func(t *testing.T) {
			f := accountHTTPTest(t)
			ctx := context.Background()
			owner := f.user(t, "LifecycleOwner", true)
			joiner := f.user(t, "LifecycleJoiner", true)
			route, err := f.routes.CreateRoute(ctx, owner.Session, routes.CreateRouteInput{Name: "Lifecycle", TransportMode: "walking", SharingPolicy: routes.SharingPolicyEveryoneCanShare})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.routes.JoinRoute(ctx, joiner.Session, route.Route.Code, routes.JoinRouteInput{TransportMode: "walking"}); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(f.handler)
			defer server.Close()
			sender := openTestAccountSocket(t, f, server.URL, joiner.Session, route.Route.Code)
			sender.event(t, "connection_established", "")
			sender.send(t, map[string]any{"type": "start_sharing", "requestId": "start"})
			sender.event(t, "command_ack", "start")
			switch action {
			case "leave":
				_, err = f.routes.LeaveRoute(ctx, joiner.Session, route.Route.Code)
			case "close":
				_, err = f.routes.UpdateRoute(ctx, owner.Session, route.Route.Code, routes.UpdateRouteInput{Status: routes.RouteStatusClosed})
			case "delete":
				err = f.routes.DeleteRoute(ctx, owner.Session, route.Route.Code)
			}
			if err != nil {
				t.Fatal(err)
			}
			sender.policyClosed(t)
			var leases, segments int
			if err = f.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM account_sharing_leases),(SELECT count(*) FROM path_segments WHERE ended_at IS NULL)`).Scan(&leases, &segments); err != nil || leases != 0 || segments != 0 {
				t.Fatal(leases, segments, err)
			}
		})
	}
}
