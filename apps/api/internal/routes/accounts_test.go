package routes

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/config"
)

func testRouteAccount(t *testing.T, pool *pgxpool.Pool, name string, verified bool) auth.Session {
	t.Helper()
	ctx := context.Background()
	var session auth.Session
	var verifiedAt *time.Time
	if verified {
		now := time.Now()
		verifiedAt = &now
	}
	err := pool.QueryRow(ctx, `INSERT INTO users(email,email_key,username,username_key,password_hash,verified_at) VALUES($1,lower($1),$2,lower($2),'test-only',$3) RETURNING id,email,username,verified_at IS NOT NULL`, name+"@example.com", name, verifiedAt).Scan(&session.User.ID, &session.User.Email, &session.User.Username, &session.User.Verified)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `INSERT INTO auth_sessions(user_id,token_hash,last_used_at,expires_at) VALUES($1,$2,now(),now()+interval '1 hour') RETURNING id,expires_at`, session.User.ID, tokenHash(name)).Scan(&session.ID, &session.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	return session
}
func accountRouteService(t *testing.T) (*AccountService, *pgxpool.Pool) {
	t.Helper()
	pool := positionTestPool(t)
	s, err := NewAccountService(pool, 10, config.DefaultPositionValidation(), 30*24*time.Hour, 30*time.Second, 7*24*time.Hour, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return s, pool
}
func createAccountRoute(t *testing.T, s *AccountService, user auth.Session, name, password string) CreateRouteResult {
	t.Helper()
	result, err := s.CreateRoute(context.Background(), user, CreateRouteInput{Name: name, Password: password, TransportMode: "walking", SharingPolicy: SharingPolicyEveryoneCanShare, Description: "private route description"})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func joinAccountRoute(t *testing.T, s *AccountService, user auth.Session, code, password string) JoinRouteResult {
	t.Helper()
	result, err := s.JoinRoute(context.Background(), user, code, JoinRouteInput{TransportMode: "bicycle", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAccountRouteAuthorityAndHistory(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	owner := testRouteAccount(t, pool, "Owner", true)
	joiner := testRouteAccount(t, pool, "Joiner", true)
	stranger := testRouteAccount(t, pool, "Stranger", true)
	pending := testRouteAccount(t, pool, "Pending", false)
	created := createAccountRoute(t, s, owner, "Protected", "routepass")
	if created.Owner.UserID != owner.User.ID || created.Owner.DisplayName != "Owner" || created.MemberToken != "" || created.OwnerToken != "" {
		t.Fatal("account authority mismatch")
	}
	if _, err := s.CreateRoute(ctx, pending, CreateRouteInput{}); !errors.Is(err, auth.ErrPending) {
		t.Fatal("pending create", err)
	}
	if _, err := s.AccessRoute(ctx, pending, created.Route.Code); !errors.Is(err, auth.ErrPending) {
		t.Fatal("pending access", err)
	}
	if _, err := s.Snapshot(ctx, stranger, created.Route.Code); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("stranger snapshot", err)
	}
	if err := s.DeleteRoute(ctx, stranger, created.Route.Code); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("stranger delete", err)
	}
	if _, err := s.UpdateRoute(ctx, stranger, created.Route.Code, UpdateRouteInput{Name: "stolen"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("stranger update", err)
	}
	if _, err := s.JoinRoute(ctx, joiner, created.Route.Code, JoinRouteInput{TransportMode: "walking", Password: "wrong"}); !errors.Is(err, ErrInvalidPassword) {
		t.Fatal("password bypass", err)
	}
	joined := joinAccountRoute(t, s, joiner, created.Route.Code, "routepass")
	again, err := s.JoinRoute(ctx, joiner, created.Route.Code, JoinRouteInput{})
	if err != nil || again.Member.ID != joined.Member.ID || !again.Restored || joined.Restored {
		t.Fatal("current membership not restored", err)
	}
	snapshot, err := s.Snapshot(ctx, joiner, created.Route.Code)
	if err != nil || snapshot.Viewer.MemberID != joined.Member.ID || len(snapshot.Members) != 2 {
		t.Fatal(snapshot, err)
	}
	for _, m := range snapshot.Members {
		if m.UserID == "" {
			t.Fatal("snapshot missing account ID")
		}
	}
	if _, err = s.LeaveRoute(ctx, owner, created.Route.Code); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("active owner left", err)
	}
	if _, err = s.LeaveRoute(ctx, joiner, created.Route.Code); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Snapshot(ctx, joiner, created.Route.Code); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("former snapshot leak", err)
	}
	if _, err = s.JoinRoute(ctx, joiner, created.Route.Code, JoinRouteInput{TransportMode: "walking"}); !errors.Is(err, ErrInvalidPassword) {
		t.Fatal("rejoin bypass", err)
	}
	rejoined := joinAccountRoute(t, s, joiner, created.Route.Code, "routepass")
	if rejoined.Member.ID == joined.Member.ID {
		t.Fatal("rejoin reused terminal episode")
	}
	snapshot, err = s.Snapshot(ctx, owner, created.Route.Code)
	if err != nil || len(snapshot.Members) != 3 {
		t.Fatal("history lost", err)
	}
	if snapshot.Members[1].Status != MemberStatusLeft {
		t.Fatal("old membership resurrected")
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("@example.com")) || bytes.Contains(raw, []byte("clientId")) || bytes.Contains(raw, []byte("memberToken")) {
		t.Fatal("route payload leaked private authority")
	}
	// Legacy mode cannot mint a member credential for this account-owned route.
	legacy := NewService(NewPostgresRepository(pool), 10)
	if _, err = legacy.AccessRoute(ctx, created.Route.Code); !errors.Is(err, ErrRouteNotFound) {
		t.Fatal("legacy access exposed account route", err)
	}
	if _, err = legacy.JoinRoute(ctx, created.Route.Code, JoinRouteInput{ClientID: "old-device", DisplayName: "Legacy", TransportMode: "walking", Password: "routepass"}); !errors.Is(err, ErrRouteNotFound) {
		t.Fatal("legacy join exposed account route", err)
	}
	// Even an accidentally retained old credential cannot authorize an account route.
	legacyToken, legacyHash, err := newOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO member_tokens(member_id,token_hash) VALUES($1,$2)`, created.Owner.ID, legacyHash); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO owner_tokens(route_id,member_id,token_hash) VALUES($1,$2,$3)`, created.Route.ID, created.Owner.ID, legacyHash); err != nil {
		t.Fatal(err)
	}
	if _, err = legacy.Snapshot(ctx, created.Route.Code, legacyToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("legacy snapshot credential bypass", err)
	}
	if err = legacy.DeleteRoute(ctx, created.Route.Code, legacyToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("legacy owner credential bypass", err)
	}
}

func TestAccountArchivedRouteAccess(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	owner := testRouteAccount(t, pool, "ArchiveOwner", true)
	user := testRouteAccount(t, pool, "ArchiveUser", true)
	created := createAccountRoute(t, s, owner, "Archive", "archivepass")
	old := joinAccountRoute(t, s, user, created.Route.Code, "archivepass")
	if _, err := s.LeaveRoute(ctx, user, created.Route.Code); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRoute(ctx, owner, created.Route.Code, UpdateRouteInput{Status: RouteStatusClosed}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Snapshot(ctx, user, created.Route.Code); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("former archive bypass", err)
	}
	current := joinAccountRoute(t, s, user, created.Route.Code, "archivepass")
	if current.Member.ID == old.Member.ID {
		t.Fatal("archive rejoin reused episode")
	}
	snapshot, err := s.Snapshot(ctx, user, created.Route.Code)
	if err != nil || snapshot.Viewer.CanStartSharing || snapshot.Viewer.CanStopSharing {
		t.Fatal("archive sharing rights", err)
	}
	if _, err = s.IssueSocketTicket(ctx, user, created.Route.Code); !errors.Is(err, ErrRouteClosed) {
		t.Fatal("archive ticket", err)
	}
	if _, err = s.LeaveRoute(ctx, owner, created.Route.Code); err != nil {
		t.Fatal("closed owner leave", err)
	}
	if _, err = s.Snapshot(ctx, owner, created.Route.Code); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("left owner read without rejoin", err)
	}
	if err = s.DeleteRoute(ctx, owner, created.Route.Code); err != nil {
		t.Fatal("owner authority lost after archive leave", err)
	}
	result, err := s.ListRoutes(ctx, user, ListRoutesInput{View: "archived", Owned: true, Joined: true, Left: true, Limit: 20})
	if err != nil || len(result.Items) != 0 {
		t.Fatal("deleted route listed", err)
	}
}

func TestAccountRouteConcurrentJoin(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	owner := testRouteAccount(t, pool, "RaceOwner", true)
	user := testRouteAccount(t, pool, "RaceUser", true)
	created := createAccountRoute(t, s, owner, "Race", "")
	gate := make(chan struct{})
	results := make(chan JoinRouteResult, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			r, err := s.JoinRoute(ctx, user, created.Route.Code, JoinRouteInput{TransportMode: "walking"})
			results <- r
			errs <- err
		}()
	}
	close(gate)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	first := ""
	for result := range results {
		if first == "" {
			first = result.Member.ID
		}
		if result.Member.ID != first {
			t.Fatal("concurrent join duplicated membership")
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM route_members WHERE user_id=$1`, user.User.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}

func TestMyRoutesFiltersPaginationAndPrivacy(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	user := testRouteAccount(t, pool, "Dashboard", true)
	owner := testRouteAccount(t, pool, "OtherOwner", true)
	mine := createAccountRoute(t, s, user, "My route", "")
	joined := createAccountRoute(t, s, owner, "Joined route", "")
	joinAccountRoute(t, s, user, joined.Route.Code, "")
	left := createAccountRoute(t, s, owner, "Left route", "")
	joinAccountRoute(t, s, user, left.Route.Code, "")
	if _, err := s.LeaveRoute(ctx, user, left.Route.Code); err != nil {
		t.Fatal(err)
	}
	archived := createAccountRoute(t, s, user, "Archived route", "")
	if _, err := s.UpdateRoute(ctx, user, archived.Route.Code, UpdateRouteInput{Status: RouteStatusClosed}); err != nil {
		t.Fatal(err)
	}
	// Equal timestamps force the ID tie-breaker to be exercised across pages.
	if _, err := pool.Exec(ctx, `UPDATE routes SET created_at='2026-10-21T12:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	for mask := 0; mask < 8; mask++ {
		input := ListRoutesInput{View: "active", Owned: mask&1 != 0, Joined: mask&2 != 0, Left: mask&4 != 0, Limit: 20}
		result, err := s.ListRoutes(ctx, user, input)
		if err != nil {
			t.Fatal(err)
		}
		expected := 0
		for bit := 0; bit < 3; bit++ {
			if mask&(1<<bit) != 0 {
				expected++
			}
		}
		if len(result.Items) != expected || result.Counts.Active != expected || result.Counts.Archived != mask&1 {
			t.Fatal("filter mismatch", mask, result)
		}
		seen := map[string]bool{}
		for _, item := range result.Items {
			if seen[item.ID] {
				t.Fatal("duplicate route")
			}
			seen[item.ID] = true
			if item.ID == left.Route.ID && (item.Relationship != "left" || item.Action != "rejoin") {
				t.Fatal("unsafe former relationship")
			}
		}
		raw, _ := json.Marshal(result)
		for _, forbidden := range []string{"private route description", "password_hash", "@example.com", "members", "points"} {
			if bytes.Contains(raw, []byte(forbidden)) {
				t.Fatal("summary leaked", forbidden)
			}
		}
	}
	input := ListRoutesInput{View: "active", Owned: true, Joined: true, Left: true, Limit: 1}
	seen := map[string]bool{}
	var firstCursor string
	for page := 0; page < 4; page++ {
		result, err := s.ListRoutes(ctx, user, input)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range result.Items {
			if seen[item.ID] {
				t.Fatal("pagination duplicate")
			}
			seen[item.ID] = true
		}
		if result.NextCursor == nil {
			break
		}
		input.Cursor = *result.NextCursor
		if firstCursor == "" {
			firstCursor = input.Cursor
		}
	}
	if len(seen) != 3 || !seen[mine.Route.ID] {
		t.Fatal("pagination omitted routes")
	}
	input.Cursor = firstCursor
	if _, err := s.ListRoutes(ctx, owner, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("cross-account cursor", err)
	}
	input.Owned = false
	if _, err := s.ListRoutes(ctx, user, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("cross-filter cursor", err)
	}
	input.Owned = true
	input.View = "archived"
	if _, err := s.ListRoutes(ctx, user, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("cross-view cursor", err)
	}
	input.View = "active"
	input.Cursor = base64.RawURLEncoding.EncodeToString([]byte("forged")) + ".bad"
	if _, err := s.ListRoutes(ctx, user, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("forged cursor", err)
	}
	// Multiple left episodes remain one summary, and explicit rejoin changes its category.
	rejoin := joinAccountRoute(t, s, user, left.Route.Code, "")
	if rejoin.Member.ID == "" {
		t.Fatal("empty episode")
	}
	input = ListRoutesInput{View: "active", Owned: false, Joined: true, Left: false, Limit: 20}
	result, err := s.ListRoutes(ctx, user, input)
	if err != nil || len(result.Items) != 2 {
		t.Fatal("rejoin category", result, err)
	}
}

func TestAccountSocketTicketsAndRevocation(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	user := testRouteAccount(t, pool, "SocketOwner", true)
	created := createAccountRoute(t, s, user, "Live", "")
	ticket, err := s.IssueSocketTicket(ctx, user, created.Route.Code)
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err = pool.QueryRow(ctx, `SELECT token_hash FROM auth_socket_tickets`).Scan(&stored); err != nil || stored == ticket.Ticket {
		t.Fatal("raw ticket stored", err)
	}
	gate := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-gate; _, err := s.ConsumeSocketTicket(ctx, ticket.Ticket); results <- err }()
	}
	close(gate)
	wg.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		} else if !errors.Is(err, ErrUnauthorized) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatal("ticket replay winners", winners)
	}
	expired, err := s.IssueSocketTicket(ctx, user, created.Route.Code)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE auth_socket_tickets SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, tokenHash(expired.Ticket)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConsumeSocketTicket(ctx, expired.Ticket); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("expired ticket", err)
	}
	fresh, err := s.IssueSocketTicket(ctx, user, created.Route.Code)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := s.ConsumeSocketTicket(ctx, fresh.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AuthorizeLive(ctx, bound); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AuthorizeLive(ctx, bound); !errors.Is(err, auth.ErrSession) {
		t.Fatal("revoked live session", err)
	}
	if _, err = s.Snapshot(ctx, user, created.Route.Code); !errors.Is(err, auth.ErrSession) {
		t.Fatal("revoked snapshot", err)
	}
	if _, err = s.StartSharing(ctx, bound); !errors.Is(err, auth.ErrSession) {
		t.Fatal("revoked sharing", err)
	}
}

func TestAccountRouteLegacyIsolation(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	user := testRouteAccount(t, pool, "LegacyCheck", true)
	old, err := NewService(NewPostgresRepository(pool), 10).CreateRoute(ctx, CreateRouteInput{ClientID: "old", DisplayName: "OldOwner", Name: "Old route", TransportMode: "walking", SharingPolicy: SharingPolicyEveryoneCanShare})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AccessRoute(ctx, user, old.Route.Code); !errors.Is(err, ErrRouteNotFound) {
		t.Fatal("unowned legacy route exposed", err)
	}
	result, err := s.ListRoutes(ctx, user, ListRoutesInput{View: "active", Owned: true, Joined: true, Left: true, Limit: 20})
	if err != nil || len(result.Items) != 0 {
		t.Fatal(result, err)
	}
	// Invalid/expired account sessions cannot borrow verified user data from the caller.
	if _, err = pool.Exec(ctx, `UPDATE auth_sessions SET expires_at=now()-interval '1 second' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateRoute(ctx, user, CreateRouteInput{}); !errors.Is(err, auth.ErrSession) {
		t.Fatal("expired session created route", err)
	}
}

func TestAccountLiveMembershipFenceAndPathHistory(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	owner := testRouteAccount(t, pool, "PathOwner", true)
	user := testRouteAccount(t, pool, "PathUser", true)
	created := createAccountRoute(t, s, owner, "Paths", "")
	joined := joinAccountRoute(t, s, user, created.Route.Code, "")
	beforeLeave, err := s.IssueSocketTicket(ctx, user, created.Route.Code)
	if err != nil {
		t.Fatal(err)
	}
	secondTicket, err := s.IssueSocketTicket(ctx, user, created.Route.Code)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := s.ConsumeSocketTicket(ctx, secondTicket.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	started, err := s.StartSharing(ctx, bound)
	if err != nil || started.Segment.ID == "" {
		t.Fatal("start path", started, err)
	}
	accuracy := 5.0
	now := time.Now()
	recorded, err := s.RecordPosition(ctx, bound, PositionUpdateInput{Latitude: 46, Longitude: 14, AccuracyM: &accuracy, ClientRecordedAt: &now})
	if err != nil || recorded.MemberID != joined.Member.ID {
		t.Fatal("account position write", recorded, err)
	}
	if _, err = s.LeaveRoute(ctx, user, created.Route.Code); err != nil {
		t.Fatal(err)
	}
	rejoined := joinAccountRoute(t, s, user, created.Route.Code, "")
	if rejoined.Member.ID == joined.Member.ID {
		t.Fatal("episode reused")
	}
	if _, err = s.ConsumeSocketTicket(ctx, beforeLeave.Ticket); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old membership ticket accepted", err)
	}
	if _, err = s.StartSharing(ctx, bound); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old binding started sharing", err)
	}
	if _, err = s.RecordPosition(ctx, bound, PositionUpdateInput{Latitude: 46, Longitude: 14, AccuracyM: &accuracy, ClientRecordedAt: &now}); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("old binding wrote position", err)
	}
	snapshot, err := s.Snapshot(ctx, user, created.Route.Code)
	if err != nil {
		t.Fatal(err)
	}
	var oldHistory *SnapshotMember
	for i := range snapshot.Members {
		if snapshot.Members[i].ID == joined.Member.ID {
			oldHistory = &snapshot.Members[i]
		}
	}
	if oldHistory == nil || oldHistory.Status != MemberStatusLeft || len(oldHistory.Paths) != 1 || len(oldHistory.Paths[0].Points) != 1 || oldHistory.Paths[0].EndedAt == nil {
		t.Fatal("historical path changed", oldHistory)
	}
	if _, err = pool.Exec(ctx, `UPDATE auth_socket_tickets SET expires_at=now()-interval '8 days'`); err != nil {
		t.Fatal(err)
	}
	if err = s.CleanupTickets(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM auth_socket_tickets`).Scan(&count); err != nil || count != 0 {
		t.Fatal("expired tickets retained", count, err)
	}
}

func TestAccountRouteMigrationRollback(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	down, err := os.ReadFile("../../../../db/migrations/0006_account_routes.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile("../../../../db/migrations/0006_account_routes.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(down)); err != nil {
		t.Fatal("empty rollback", err)
	}
	legacy := NewService(NewPostgresRepository(pool), 10)
	old, err := legacy.CreateRoute(ctx, CreateRouteInput{Name: "Legacy after rollback", ClientID: "legacy", DisplayName: "Legacy", TransportMode: "walking", SharingPolicy: SharingPolicyEveryoneCanShare})
	if err != nil {
		t.Fatal("legacy mode without migration 0006", err)
	}
	if _, err = legacy.Snapshot(ctx, old.Route.Code, old.MemberToken); err != nil {
		t.Fatal("legacy snapshot without migration 0006", err)
	}
	if _, err = pool.Exec(ctx, string(up)); err != nil {
		t.Fatal("reapply migration", err)
	}
	user := testRouteAccount(t, pool, "RollbackOwner", true)
	created := createAccountRoute(t, s, user, "Retain authority", "")
	if _, err = pool.Exec(ctx, string(down)); err == nil {
		t.Fatal("rollback silently removed account authority")
	}
	if _, err = s.Snapshot(ctx, user, created.Route.Code); err != nil {
		t.Fatal("failed rollback damaged route", err)
	}
}
