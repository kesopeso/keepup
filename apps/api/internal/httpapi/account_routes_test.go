package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/config"
	"keepup/apps/api/internal/routes"
)

type accountHTTPFixture struct {
	pool     *pgxpool.Pool
	accounts *auth.Service
	routes   *routes.AccountService
	handler  http.Handler
}

func accountHTTPTest(t *testing.T) accountHTTPFixture {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for account route HTTP integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	for _, extension := range []string{"postgis", "pgcrypto"} {
		if _, err = admin.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS "+extension+" WITH SCHEMA public"); err != nil {
			t.Fatal(err)
		}
	}
	schema := fmt.Sprintf("account_http_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	pgcfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	pgcfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, pgcfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files, err := filepath.Glob("../../../../db/migrations/*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		sql, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	recovery := auth.RecoveryConfig{PublicURL: "https://keepup.example", EncryptionKey: bytes.Repeat([]byte{1}, 32), Policy: auth.DefaultRecoveryPolicy()}
	policy := auth.DefaultPolicy()
	policy.SignupAttempts = 1000
	policy.LoginAttempts = 1000
	recovery.Policy.MailPerSource = 1000
	accountRoutes, err := routes.NewAccountService(pool, 10, config.DefaultPositionValidation(), policy.SessionIdle, 30*time.Second, recovery.Policy.Retention, recovery.EncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := auth.NewService(auth.NewPostgresRepository(pool), policy, recovery, auth.WithSessionRevoker(accountRoutes))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.AuthConfig{Origins: []string{"https://keepup.example"}, Policy: policy, SocketTicketTTL: 30 * time.Second}
	app := config.AppConfig{Env: "production", WebSocketPingInterval: 50 * time.Millisecond, WebSocketPingTimeout: time.Second, TrackingStaleAfter: time.Hour, TrackingOfflineAfter: time.Hour, SpectatorOfflineAfter: time.Millisecond}
	handler := NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), app, pool, routes.NewService(routes.NewPostgresRepository(pool), 10), WithAccountRoutes(accountRoutes, accounts, cfg))
	return accountHTTPFixture{pool, accounts, accountRoutes, handler}
}
func (f accountHTTPFixture) user(t *testing.T, name string, verified bool) auth.Result {
	t.Helper()
	ctx := context.Background()
	result, err := f.accounts.Signup(ctx, auth.SignupInput{Email: name + "@example.com", Username: name, Password: "password1!"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if verified {
		if _, err = f.pool.Exec(ctx, `UPDATE users SET verified_at=now() WHERE id=$1`, result.Session.User.ID); err != nil {
			t.Fatal(err)
		}
		result.Session, err = f.accounts.Authenticate(ctx, result.Token)
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}
func accountRouteRequest(handler http.Handler, method, path, body, token string, native bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if native {
		req.Header.Set("X-KeepUp-Client", "native")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	} else {
		req.Header.Set("Origin", "https://keepup.example")
		req.Header.Set("X-KeepUp-Request", "1")
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "__Host-keepup_session", Value: token})
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestAccountRouteHTTPTransportsAndIsolation(t *testing.T) {
	f := accountHTTPTest(t)
	owner := f.user(t, "HTTPowner", true)
	other := f.user(t, "HTTPother", true)
	pending := f.user(t, "HTTPpending", false)
	body := `{"name":"Private","description":"route description","transportMode":"walking","sharingPolicy":"everyone_can_share","password":"routepass"}`
	w := accountRouteRequest(f.handler, "POST", "/routes", body, owner.Token, true)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "memberToken") || strings.Contains(w.Body.String(), "ownerToken") || strings.Contains(w.Body.String(), "clientId") || strings.Contains(w.Body.String(), "@example.com") {
		t.Fatal("route response leaked account/private authority", w.Body.String())
	}
	var created routes.CreateRouteResult
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Owner.DisplayName != "HTTPowner" || created.Owner.UserID != owner.Session.User.ID {
		t.Fatal("display not account-derived")
	}
	for _, path := range []string{"/routes/" + created.Route.Code, "/routes/" + created.Route.Code + "/access", "/me/routes"} {
		for _, native := range []bool{false, true} {
			unauth := accountRouteRequest(f.handler, "GET", path, "", "", native)
			if unauth.Code != 401 {
				t.Fatal("missing session", path, unauth.Code)
			}
			blocked := accountRouteRequest(f.handler, "GET", path, "", pending.Token, native)
			if blocked.Code != 403 || !strings.Contains(blocked.Body.String(), "email_verification_required") {
				t.Fatal("pending account gate", path, blocked.Code, blocked.Body.String())
			}
			old := accountRouteRequest(f.handler, "GET", path, "", strings.Repeat("a", 64), native)
			if old.Code != 401 {
				t.Fatal("legacy credential accepted", path, old.Code)
			}
		}
	}
	access := accountRouteRequest(f.handler, "GET", "/routes/"+created.Route.Code+"/access", "", other.Token, false)
	if access.Code != 200 {
		t.Fatal("active access metadata denied", access.Code)
	}
	snapshot := accountRouteRequest(f.handler, "GET", "/routes/"+created.Route.Code, "", other.Token, true)
	if snapshot.Code != 403 {
		t.Fatal("nonmember snapshot", snapshot.Code)
	}
	update := accountRouteRequest(f.handler, "PATCH", "/routes/"+created.Route.Code, `{"name":"stolen"}`, other.Token, true)
	if update.Code != 403 {
		t.Fatal("nonowner update", update.Code)
	}
	for _, legacyBody := range []string{`{"clientId":"","displayName":"","transportMode":"walking"}`, `{"clientId":"old","transportMode":"walking"}`} {
		blocked := accountRouteRequest(f.handler, "POST", "/routes/"+created.Route.Code+"/members", legacyBody, other.Token, true)
		if blocked.Code != 400 {
			t.Fatal("legacy join fields accepted", blocked.Code)
		}
	}
	rejectedCreate := accountRouteRequest(f.handler, "POST", "/routes", strings.TrimSuffix(body, "}")+`,"clientId":""}`, owner.Token, true)
	if rejectedCreate.Code != 400 {
		t.Fatal("legacy create accepted", rejectedCreate.Code)
	}
	joined := accountRouteRequest(f.handler, "POST", "/routes/"+created.Route.Code+"/members", `{"transportMode":"bicycle","password":"routepass"}`, other.Token, false)
	if joined.Code != 201 {
		t.Fatal(joined.Code, joined.Body.String())
	}
	// A separate account session restores the same membership without route credentials.
	second, err := f.accounts.Login(context.Background(), "HTTPother", "password1!", "test")
	if err != nil {
		t.Fatal(err)
	}
	restored := accountRouteRequest(f.handler, "GET", "/routes/"+created.Route.Code, "", second.Token, true)
	if restored.Code != 200 {
		t.Fatal(restored.Code, restored.Body.String())
	}
	if restored.Header().Get("Cache-Control") != "no-store" || restored.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("private response CORS/cache policy")
	}
	for _, query := range []string{"owned=1", "joined=TRUE", "left=", "view=closed", "limit=0", "limit=101", "limit=bad", "owned=true&owned=false", "cursor=", "cursor=forged", "unexpected=true", "view=%zz"} {
		invalid := accountRouteRequest(f.handler, "GET", "/me/routes?"+query, "", owner.Token, true)
		if invalid.Code != 400 {
			t.Fatal("bad query accepted", query, invalid.Code, invalid.Body.String())
		}
	}
	empty := accountRouteRequest(f.handler, "GET", "/me/routes?owned=false&joined=false&left=false", "", owner.Token, false)
	if empty.Code != 200 || !strings.Contains(empty.Body.String(), `"items":[]`) || !strings.Contains(empty.Body.String(), `"nextCursor":null`) {
		t.Fatal(empty.Code, empty.Body.String())
	}
	// Missing CSRF header must fail before cookie credentials can authorize a mutation.
	req := httptest.NewRequest("POST", "/routes", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://keepup.example")
	req.AddCookie(&http.Cookie{Name: "__Host-keepup_session", Value: owner.Token})
	csrf := httptest.NewRecorder()
	f.handler.ServeHTTP(csrf, req)
	if csrf.Code != 403 {
		t.Fatal("CSRF guard missing")
	}
}

func TestAccountSocketHTTPBootstrapAndRevocation(t *testing.T) {
	f := accountHTTPTest(t)
	owner := f.user(t, "SocketHTTP", true)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	created, err := f.routes.CreateRoute(ctx, owner.Session, routes.CreateRouteInput{Name: "Live", TransportMode: "walking", SharingPolicy: routes.SharingPolicyEveryoneCanShare})
	if err != nil {
		t.Fatal(err)
	}
	ticketResponse := accountRouteRequest(f.handler, "POST", "/routes/"+created.Route.Code+"/socket-ticket", "{}", owner.Token, false)
	if ticketResponse.Code != 200 {
		t.Fatal(ticketResponse.Code, ticketResponse.Body.String())
	}
	var ticket routes.SocketTicket
	if err = json.Unmarshal(ticketResponse.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.handler)
	defer server.Close()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	options := &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://keepup.example"}}}
	connection, _, err := websocket.Dial(ctx, endpoint, options)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.CloseNow() }()
	if err = wsjson.Write(ctx, connection, map[string]any{"type": "authenticate", "ticket": ticket.Ticket}); err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if err = wsjson.Read(ctx, connection, &event); err != nil || event["type"] != "connection_established" {
		t.Fatal(event, err)
	}
	replay, _, err := websocket.Dial(ctx, endpoint, options)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = replay.CloseNow() }()
	if err = wsjson.Write(ctx, replay, map[string]any{"type": "authenticate", "ticket": ticket.Ticket}); err != nil {
		t.Fatal(err)
	}
	if err = wsjson.Read(ctx, replay, &event); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatal("ticket replay accepted", err)
	}
	old, _, err := websocket.Dial(ctx, endpoint, options)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = old.CloseNow() }()
	if err = wsjson.Write(ctx, old, map[string]any{"type": "authenticate", "memberToken": "old-token"}); err != nil {
		t.Fatal(err)
	}
	if err = wsjson.Read(ctx, old, &event); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatal("legacy frame accepted", err)
	}
	bad, reply, err := websocket.Dial(ctx, endpoint+"?ticket=secret", options)
	if bad != nil {
		_ = bad.CloseNow()
	}
	if err == nil || reply == nil || reply.StatusCode != 400 {
		t.Fatal("URL credential accepted", err)
	}
	bad, reply, err = websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"https://evil.example"}}})
	if bad != nil {
		_ = bad.CloseNow()
	}
	if err == nil || reply == nil || reply.StatusCode != 403 {
		t.Fatal("untrusted socket origin accepted", err)
	}
	closed := make(chan error, 1)
	go func() {
		for {
			var event map[string]any
			if err := wsjson.Read(ctx, connection, &event); err != nil {
				closed <- err
				return
			}
		}
	}()
	if err = f.accounts.Logout(ctx, owner.Token, false); err != nil {
		t.Fatal(err)
	}
	if err = <-closed; websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatal("revoked socket retained data access", err)
	}
}
