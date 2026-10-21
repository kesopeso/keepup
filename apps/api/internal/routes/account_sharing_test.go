package routes

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"keepup/apps/api/internal/auth"
)

func openAccountLive(t *testing.T, s *AccountService, session auth.Session, code string) LiveAccount {
	t.Helper()
	ctx := context.Background()
	ticket, err := s.IssueSocketTicket(ctx, session, code)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := s.ConsumeSocketTicket(ctx, ticket.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	bound, err = s.OpenLive(ctx, bound)
	if err != nil {
		t.Fatal(err)
	}
	return bound
}

func startAccountLive(t *testing.T, s *AccountService, bound LiveAccount) LiveAccount {
	t.Helper()
	result, err := s.StartSharing(context.Background(), bound)
	if err != nil || result.SharingGeneration == "" || !result.Changed {
		t.Fatal(result, err)
	}
	bound.SharingGeneration = result.SharingGeneration
	return bound
}

func livePoint() PositionUpdateInput {
	accuracy := 5.0
	now := time.Now()
	return PositionUpdateInput{Latitude: 46, Longitude: 14, AccuracyM: &accuracy, ClientRecordedAt: &now}
}

func TestAccountSharingConcurrentStartAndFencing(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	owner := testRouteAccount(t, pool, "ConcurrentSender", true)
	route := createAccountRoute(t, s, owner, "Concurrency", "")
	a := openAccountLive(t, s, owner, route.Route.Code)
	b := openAccountLive(t, s, owner, route.Route.Code)
	if a.ConnectionID == b.ConnectionID {
		t.Fatal("connections reused")
	}
	type outcome struct {
		bound  LiveAccount
		result AccountSharingResult
		err    error
	}
	results := make(chan outcome, 2)
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for _, bound := range []LiveAccount{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			r, err := s.StartSharing(ctx, bound)
			results <- outcome{bound, r, err}
		}()
	}
	close(gate)
	wg.Wait()
	close(results)
	var sender, viewer LiveAccount
	winners := 0
	for result := range results {
		if result.err == nil {
			winners++
			sender = result.bound
			sender.SharingGeneration = result.result.SharingGeneration
		} else {
			if !errors.Is(result.err, ErrSharingOnAnotherDevice) {
				t.Fatal(result.err)
			}
			viewer = result.bound
		}
	}
	if winners != 1 {
		t.Fatal("concurrent winners", winners)
	}
	retry, err := s.StartSharing(ctx, sender)
	if err != nil || retry.Changed || retry.SharingGeneration != sender.SharingGeneration {
		t.Fatal("retry changed lease", retry, err)
	}
	for _, tt := range []struct {
		name  string
		bound LiveAccount
		stop  bool
	}{
		{"viewer knows generation", LiveAccount{Session: viewer.Session, Code: viewer.Code, MemberID: viewer.MemberID, ConnectionID: viewer.ConnectionID, SharingGeneration: sender.SharingGeneration}, true},
		{"missing generation", LiveAccount{Session: sender.Session, Code: sender.Code, MemberID: sender.MemberID, ConnectionID: sender.ConnectionID}, false},
		{"wrong generation", LiveAccount{Session: sender.Session, Code: sender.Code, MemberID: sender.MemberID, ConnectionID: sender.ConnectionID, SharingGeneration: "old"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.stop {
				_, err = s.StopSharing(ctx, tt.bound)
			} else {
				_, err = s.RecordPosition(ctx, tt.bound, livePoint())
			}
			if !errors.Is(err, ErrSharingExpired) {
				t.Fatal(err)
			}
		})
	}
	if _, err = s.RecordPosition(ctx, sender, livePoint()); err != nil {
		t.Fatal(err)
	}
	viewerCaps, err := s.LiveViewer(ctx, viewer)
	if err != nil || viewerCaps.CanStopSharing || viewerCaps.CanStartSharing {
		t.Fatal(viewerCaps, err)
	}
	senderCaps, err := s.LiveViewer(ctx, sender)
	if err != nil || !senderCaps.CanStopSharing {
		t.Fatal(senderCaps, err)
	}
	snapshot, err := s.Snapshot(ctx, owner, route.Route.Code)
	if err != nil || snapshot.Viewer.CanStopSharing {
		t.Fatal("REST claimed connection authority", err)
	}
	if _, err = s.StopSharing(ctx, sender); err != nil {
		t.Fatal(err)
	}
	next := startAccountLive(t, s, viewer)
	if next.SharingGeneration == sender.SharingGeneration {
		t.Fatal("generation reused")
	}
	if _, err = s.RecordPosition(ctx, sender, livePoint()); !errors.Is(err, ErrSharingExpired) {
		t.Fatal("late old sender point", err)
	}
	if _, err = s.StopSharing(ctx, sender); !errors.Is(err, ErrSharingExpired) {
		t.Fatal("late old sender stop", err)
	}
	// A disconnect from the old sender cannot mark the replacement sender stale.
	if err = s.DisconnectLive(ctx, sender); err != nil {
		t.Fatal(err)
	}
	current, err := s.AuthorizeLive(ctx, next)
	if err != nil || current.Member.Status != MemberStatusTracking {
		t.Fatal(current, err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM path_segments WHERE member_id=$1 AND ended_at IS NULL`, sender.MemberID).Scan(&count); err != nil || count != 1 {
		t.Fatal("open segments", count, err)
	}
	// The same account can independently send on another route/device.
	second := createAccountRoute(t, s, owner, "Second", "")
	_ = startAccountLive(t, s, openAccountLive(t, s, owner, second.Route.Code))
}

func TestAccountSharingSenderTimeoutIgnoresViewers(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	owner := testRouteAccount(t, pool, "TimeoutOwner", true)
	route := createAccountRoute(t, s, owner, "Timeout", "")
	sender := startAccountLive(t, s, openAccountLive(t, s, owner, route.Route.Code))
	viewer := openAccountLive(t, s, owner, route.Route.Code)
	// A bad GPS point must not renew the lease deadline.
	if _, err := pool.Exec(ctx, `UPDATE account_sharing_leases SET last_position_at=now()-interval '1 minute'`); err != nil {
		t.Fatal(err)
	}
	bad := livePoint()
	bad.Latitude = 999
	if _, err := s.RecordPosition(ctx, sender, bad); err == nil {
		t.Fatal("bad GPS accepted")
	}
	for i := 0; i < 3; i++ {
		if _, err := s.AuthorizeLive(ctx, viewer); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SweepLive(ctx); err != nil {
		t.Fatal(err)
	}
	current, err := s.AuthorizeLive(ctx, viewer)
	if err != nil || current.Member.Status != MemberStatusStale {
		t.Fatal("viewer refreshed sender", current, err)
	}
	if _, err = s.StartSharing(ctx, viewer); !errors.Is(err, ErrSharingOnAnotherDevice) {
		t.Fatal("stale lease taken over", err)
	}
	if _, err = s.RecordPosition(ctx, sender, livePoint()); err != nil {
		t.Fatal("sender failed to recover", err)
	}
	if err = s.DisconnectLive(ctx, sender); err != nil {
		t.Fatal(err)
	}
	// Reconnection is a viewer and cannot adopt the old connection's generation.
	reconnected := openAccountLive(t, s, owner, route.Route.Code)
	reconnected.SharingGeneration = sender.SharingGeneration
	if _, err = s.RecordPosition(ctx, reconnected, livePoint()); !errors.Is(err, ErrSharingExpired) {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE account_sharing_leases SET stale_at=now()-interval '6 minutes'`); err != nil {
		t.Fatal(err)
	}
	if err = s.SweepLive(ctx); err != nil {
		t.Fatal(err)
	}
	current, err = s.AuthorizeLive(ctx, viewer)
	if err != nil || current.Member.Status != MemberStatusSpectating {
		t.Fatal("viewer lost presence", current, err)
	}
	next := startAccountLive(t, s, viewer)
	if err = s.SweepLive(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecordPosition(ctx, next, livePoint()); err != nil {
		t.Fatal("old deadline ended new lease", err)
	}
	if _, err = s.StopSharing(ctx, next); err != nil {
		t.Fatal(err)
	}
	for _, bound := range []LiveAccount{viewer, reconnected} {
		if err = s.DisconnectLive(ctx, bound); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `UPDATE route_members SET live_absent_since=now()-interval '1 minute' WHERE id=$1`, viewer.MemberID); err != nil {
		t.Fatal(err)
	}
	if err = s.SweepLive(ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM route_members WHERE id=$1`, viewer.MemberID).Scan(&status); err != nil || status != MemberStatusOffline {
		t.Fatal(status, err)
	}
}

func TestAccountSharingCapacityAndLifecycle(t *testing.T) {
	for _, action := range []string{"leave", "close", "delete", "expiry"} {
		t.Run(action, func(t *testing.T) {
			s, pool := accountRouteService(t)
			ctx := context.Background()
			owner := testRouteAccount(t, pool, "CapacityOwner", true)
			user := testRouteAccount(t, pool, "CapacityUser", true)
			route := createAccountRoute(t, s, owner, "Capacity", "")
			joinAccountRoute(t, s, user, route.Route.Code, "")
			if _, err := pool.Exec(ctx, `UPDATE routes SET max_tracking_members=1 WHERE id=$1`, route.Route.ID); err != nil {
				t.Fatal(err)
			}
			a := openAccountLive(t, s, owner, route.Route.Code)
			b := startAccountLive(t, s, openAccountLive(t, s, user, route.Route.Code))
			if _, err := s.StartSharing(ctx, a); !errors.Is(err, ErrTrackingLimitReached) {
				t.Fatal("capacity", err)
			}
			var err error
			switch action {
			case "leave":
				_, err = s.LeaveRoute(ctx, user, route.Route.Code)
			case "close":
				_, err = s.UpdateRoute(ctx, owner, route.Route.Code, UpdateRouteInput{Status: RouteStatusClosed})
			case "delete":
				err = s.DeleteRoute(ctx, owner, route.Route.Code)
			case "expiry":
				_, err = pool.Exec(ctx, `UPDATE auth_sessions SET expires_at=now()-interval '1 second' WHERE id=$1`, user.ID)
				if err == nil {
					err = s.SweepLive(ctx)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			var leases, segments, connections int
			if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM account_sharing_leases WHERE member_id=$1),(SELECT count(*) FROM path_segments WHERE member_id=$1 AND ended_at IS NULL),(SELECT count(*) FROM account_live_connections WHERE member_id=$1)`, b.MemberID).Scan(&leases, &segments, &connections); err != nil || leases != 0 || segments != 0 || connections != 0 {
				t.Fatal(leases, segments, connections, err)
			}
			if _, err = s.RecordPosition(ctx, b, livePoint()); err == nil {
				t.Fatal("terminal sender accepted")
			}
			if action == "leave" || action == "expiry" {
				_ = startAccountLive(t, s, a)
			}
		})
	}
}

func TestAccountLiveStartupRecoveryAndSingleton(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	owner := testRouteAccount(t, pool, "RestartOwner", true)
	route := createAccountRoute(t, s, owner, "Restart", "")
	old := startAccountLive(t, s, openAccountLive(t, s, owner, route.Route.Code))
	legacy := NewService(NewPostgresRepository(pool), 10)
	oldRoute, err := legacy.CreateRoute(ctx, CreateRouteInput{Name: "Legacy", ClientID: "old", DisplayName: "Old", TransportMode: "walking", SharingPolicy: SharingPolicyEveryoneCanShare})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = legacy.StartSharing(ctx, oldRoute.Route.Code, oldRoute.MemberToken); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stop, err := s.StartLive(ctx, logger, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if _, err = s.StartLive(ctx, logger, nil); err == nil {
		t.Fatal("second runtime acquired lock")
	}
	if _, err = s.AuthorizeLive(ctx, old); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("abandoned connection survived", err)
	}
	var open int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM path_segments WHERE member_id=$1 AND ended_at IS NULL`, old.MemberID).Scan(&open); err != nil || open != 0 {
		t.Fatal(open, err)
	}
	preserved, err := legacy.AuthorizeMember(ctx, oldRoute.MemberToken)
	if err != nil || preserved.Member.Status != MemberStatusTracking {
		t.Fatal("recovery touched legacy", preserved, err)
	}
	next := openAccountLive(t, s, owner, route.Route.Code)
	current, err := s.AuthorizeLive(ctx, next)
	if err != nil || current.Member.Status != MemberStatusSpectating {
		t.Fatal("reconnect automatically shared", current, err)
	}
	_ = startAccountLive(t, s, next)
}

func TestAccountSharingDifferentUsersRaceForLastSlot(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	owner := testRouteAccount(t, pool, "SlotOwner", true)
	joiner := testRouteAccount(t, pool, "SlotJoiner", true)
	route := createAccountRoute(t, s, owner, "Last slot", "")
	joinAccountRoute(t, s, joiner, route.Route.Code, "")
	if _, err := pool.Exec(ctx, `UPDATE routes SET max_tracking_members=1 WHERE id=$1`, route.Route.ID); err != nil {
		t.Fatal(err)
	}
	a, b := openAccountLive(t, s, owner, route.Route.Code), openAccountLive(t, s, joiner, route.Route.Code)
	gate := make(chan struct{})
	results := make(chan error, 2)
	for _, bound := range []LiveAccount{a, b} {
		go func() { <-gate; _, err := s.StartSharing(ctx, bound); results <- err }()
	}
	close(gate)
	winners := 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			winners++
		} else if !errors.Is(err, ErrTrackingLimitReached) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatal(winners)
	}
}

func TestAccountLiveLockLossFencesOperationsBeforeNextTick(t *testing.T) {
	s, pool := accountRouteService(t)
	ctx := context.Background()
	owner := testRouteAccount(t, pool, "LostRuntime", true)
	route := createAccountRoute(t, s, owner, "Lost runtime", "")
	lost := make(chan struct{})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stop, err := s.StartLive(ctx, logger, func() { close(lost) })
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	// Terminate only this test schema's dedicated runtime backend.
	if _, err = pool.Exec(ctx, `SELECT pg_terminate_backend($1)`, s.livePID.Load()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.IssueSocketTicket(ctx, owner, route.Route.Code); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("lost lock still grants authority", err)
	}
	select {
	case <-lost:
	case <-time.After(5 * time.Second):
		t.Fatal("runtime loss did not request shutdown")
	}
}
