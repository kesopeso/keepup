package routes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/config"
)

var (
	// ErrSharingOnAnotherDevice rejects takeover of an existing sender.
	ErrSharingOnAnotherDevice = errors.New("sharing on another device")
	// ErrSharingExpired rejects missing, released or obsolete sender generations.
	ErrSharingExpired = errors.New("sharing expired")
)

func (s *AccountService) checkLiveRuntime(ctx context.Context, tx pgx.Tx) error {
	pid := s.livePID.Load()
	if pid == 0 {
		return nil
	} // Isolated service tests need no running deadline worker.
	var held bool
	// The dedicated backend owns exactly one advisory lock and runs no other work.
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND mode='ExclusiveLock' AND granted)`, pid).Scan(&held)
	if err != nil {
		return err
	}
	if !held {
		s.liveStopped.Store(true)
		return ErrUnauthorized
	}
	return nil
}

// ConfigureLive installs timings and process-local observers before serving.
func (s *AccountService) ConfigureLive(cfg config.AppConfig, event func(string, string, Member), revoked func(auth.Revocation), lost func(string, string, *Route)) {
	if cfg.TrackingStaleAfter > 0 {
		s.liveConfig.TrackingStaleAfter = cfg.TrackingStaleAfter
	}
	if cfg.TrackingOfflineAfter > 0 {
		s.liveConfig.TrackingOfflineAfter = cfg.TrackingOfflineAfter
	}
	if cfg.SpectatorOfflineAfter > 0 {
		s.liveConfig.SpectatorOfflineAfter = cfg.SpectatorOfflineAfter
	}
	s.liveEvent, s.liveRevoked, s.liveAccessLost = event, revoked, lost
}

func liveConnection(ctx context.Context, tx pgx.Tx, bound LiveAccount) error {
	if bound.ConnectionID == "" {
		return ErrUnauthorized
	}
	var found bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_live_connections WHERE id::text=$1 AND session_id=$2 AND member_id=$3)`, bound.ConnectionID, bound.Session.ID, bound.MemberID).Scan(&found)
	if err != nil {
		return err
	}
	if !found {
		return ErrUnauthorized
	}
	return nil
}

func (s *AccountService) senderLease(ctx context.Context, tx pgx.Tx, bound LiveAccount) error {
	if err := liveConnection(ctx, tx, bound); err != nil {
		return err
	}
	var valid bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_sharing_leases WHERE member_id=$1 AND session_id=$2 AND connection_id::text=$3 AND generation::text=$4
        AND COALESCE(stale_at,last_position_at+$5*interval '1 second')>clock_timestamp()-$6*interval '1 second')`, bound.MemberID, bound.Session.ID, bound.ConnectionID, bound.SharingGeneration, s.liveConfig.TrackingStaleAfter.Seconds(), s.liveConfig.TrackingOfflineAfter.Seconds()).Scan(&valid)
	if err != nil {
		return err
	}
	if !valid {
		return ErrSharingExpired
	}
	return nil
}

// OpenLive registers an independent viewer and restores presence, never sharing.
func (s *AccountService) OpenLive(ctx context.Context, bound LiveAccount) (LiveAccount, error) {
	return accountOperation(ctx, s, bound.Session, bound.Code, true, false, func(tx pgx.Tx, core *Service, a accountRoute, _ auth.User) (LiveAccount, error) {
		if a.Member.ID != bound.MemberID {
			return LiveAccount{}, ErrUnauthorized
		}
		if a.Route.Status != RouteStatusActive {
			return LiveAccount{}, ErrRouteClosed
		}
		err := tx.QueryRow(ctx, `INSERT INTO account_live_connections(session_id,route_id,member_id) VALUES($1,$2,$3) RETURNING id`, bound.Session.ID, a.Route.ID, a.Member.ID).Scan(&bound.ConnectionID)
		if err != nil {
			return LiveAccount{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE route_members SET live_absent_since=NULL WHERE id=$1`, a.Member.ID); err != nil {
			return LiveAccount{}, err
		}
		_, bound.PresenceRestored, err = core.MarkMemberOnline(ctx, a.Route.ID, a.Member.ID)
		return bound, err
	})
}

// LiveViewer returns capabilities for this connection without exposing lease secrets.
func (s *AccountService) LiveViewer(ctx context.Context, bound LiveAccount) (ViewerCapabilities, error) {
	return accountOperation(ctx, s, bound.Session, bound.Code, true, false, func(tx pgx.Tx, core *Service, a accountRoute, _ auth.User) (ViewerCapabilities, error) {
		if a.Member.ID != bound.MemberID {
			return ViewerCapabilities{}, ErrUnauthorized
		}
		if err := liveConnection(ctx, tx, bound); err != nil {
			return ViewerCapabilities{}, err
		}
		count, err := core.repo.CountTrackingMembers(ctx, a.Route.ID)
		if err != nil {
			return ViewerCapabilities{}, err
		}
		viewer := buildViewerCapabilities(a.AuthorizedMember, count)
		var connection string
		err = tx.QueryRow(ctx, `SELECT connection_id FROM account_sharing_leases WHERE member_id=$1`, bound.MemberID).Scan(&connection)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return viewer, err
		}
		viewer.CanStartSharing = viewer.CanStartSharing && connection == ""
		viewer.CanStopSharing = viewer.CanStopSharing && connection == bound.ConnectionID
		return viewer, nil
	})
}

type presenceChange struct {
	member Member
	kind   string
}

func liveMember(ctx context.Context, tx pgx.Tx, memberID string) (Member, error) {
	var m Member
	err := tx.QueryRow(ctx, `SELECT m.id,m.route_id,u.id,u.username,m.transport_mode,m.is_owner,m.status,m.color,m.joined_at,m.left_at FROM route_members m JOIN users u ON u.id=m.user_id WHERE m.id=$1`, memberID).Scan(&m.ID, &m.RouteID, &m.UserID, &m.DisplayName, &m.TransportMode, &m.IsOwner, &m.Status, &m.Color, &m.JoinedAt, &m.LeftAt)
	return m, err
}

func (s *AccountService) publishPresence(changes []presenceChange) {
	if s.liveEvent == nil {
		return
	}
	for _, change := range changes {
		s.liveEvent(change.member.RouteID, change.kind, change.member)
	}
}

// DisconnectLive removes only this viewer. A sender becomes stale even if viewers remain.
func (s *AccountService) DisconnectLive(ctx context.Context, bound LiveAccount) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var routeID string
	err = tx.QueryRow(ctx, `SELECT id FROM routes WHERE code=$1 AND owner_user_id IS NOT NULL FOR UPDATE`, bound.Code).Scan(&routeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM account_live_connections WHERE id::text=$1 AND member_id=$2 AND session_id=$3`, bound.ConnectionID, bound.MemberID, bound.Session.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE route_members SET live_absent_since=COALESCE(live_absent_since,clock_timestamp()) WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM account_live_connections WHERE member_id=$1)`, bound.MemberID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE account_sharing_leases SET stale_at=clock_timestamp() WHERE member_id=$1 AND connection_id::text=$2 AND stale_at IS NULL`, bound.MemberID, bound.ConnectionID)
	if err != nil {
		return err
	}
	var changes []presenceChange
	if tag.RowsAffected() > 0 {
		if _, err = tx.Exec(ctx, `UPDATE route_members SET status='stale' WHERE id=$1 AND status='tracking'`, bound.MemberID); err != nil {
			return err
		}
		m, err := liveMember(ctx, tx, bound.MemberID)
		if err != nil {
			return err
		}
		changes = append(changes, presenceChange{m, "member_became_stale"})
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.publishPresence(changes)
	return nil
}

// finishLease releases capacity and closes the segment while the route is locked.
func finishLease(ctx context.Context, tx pgx.Tx, memberID string) (presenceChange, error) {
	if _, err := tx.Exec(ctx, `DELETE FROM account_sharing_leases WHERE member_id=$1`, memberID); err != nil {
		return presenceChange{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE path_segments SET ended_at=clock_timestamp(),end_reason='disconnected' WHERE member_id=$1 AND ended_at IS NULL`, memberID); err != nil {
		return presenceChange{}, err
	}
	_, err := tx.Exec(ctx, `UPDATE route_members SET status=CASE WHEN EXISTS(SELECT 1 FROM account_live_connections WHERE member_id=$1) THEN 'spectating' ELSE 'offline' END WHERE id=$1 AND left_at IS NULL`, memberID)
	if err != nil {
		return presenceChange{}, err
	}
	m, err := liveMember(ctx, tx, memberID)
	kind := "member_stopped_sharing"
	if m.Status == MemberStatusOffline {
		kind = "member_went_offline"
	}
	return presenceChange{m, kind}, err
}

// Revoke implements auth.SessionRevoker in the same transaction as session revocation.
func (s *AccountService) Revoke(ctx context.Context, tx pgx.Tx, change *auth.Revocation) error {
	// Freeze the session set under the caller's user lock. A fresh login after
	// commit must not be caught by a delayed logout-all notification.
	sessions, err := tx.Query(ctx, `SELECT id::text FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NOT NULL AND ($2='' OR id::text=$2) AND ($3='' OR id::text<>$3)`, change.UserID, change.SessionID, change.ExceptSessionID)
	if err != nil {
		return err
	}
	change.SessionIDs, err = pgx.CollectRows(sessions, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if change.SessionIDs == nil {
		change.SessionIDs = []string{}
	}
	if _, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_reason=$4 WHERE user_id=$1 AND revoked_at IS NOT NULL AND ($2='' OR id::text=$2) AND ($3='' OR id::text<>$3) AND revoked_reason IS NULL`, change.UserID, change.SessionID, change.ExceptSessionID, change.Reason); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT route_id::text FROM account_live_connections WHERE session_id IN (SELECT id FROM auth_sessions WHERE user_id=$1 AND ($2='' OR id::text=$2) AND ($3='' OR id::text<>$3))
        UNION SELECT route_id::text FROM account_sharing_leases WHERE user_id=$1 AND ($2='' OR session_id::text=$2) AND ($3='' OR session_id::text<>$3) ORDER BY 1`, change.UserID, change.SessionID, change.ExceptSessionID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err = tx.Exec(ctx, `SELECT id FROM routes WHERE id=$1 FOR UPDATE`, id); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM account_live_connections WHERE route_id=$1 AND session_id IN(SELECT id FROM auth_sessions WHERE user_id=$2 AND ($3='' OR id::text=$3) AND ($4='' OR id::text<>$4))`, id, change.UserID, change.SessionID, change.ExceptSessionID); err != nil {
			return err
		}
		var memberID string
		err = tx.QueryRow(ctx, `SELECT member_id FROM account_sharing_leases WHERE route_id=$1 AND user_id=$2 AND ($3='' OR session_id::text=$3) AND ($4='' OR session_id::text<>$4)`, id, change.UserID, change.SessionID, change.ExceptSessionID).Scan(&memberID)
		if err == nil {
			if _, err = finishLease(ctx, tx, memberID); err != nil {
				return err
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE route_members SET live_absent_since=COALESCE(live_absent_since,clock_timestamp()) WHERE route_id=$1 AND user_id=$2 AND NOT EXISTS(SELECT 1 FROM account_live_connections c WHERE c.member_id=route_members.id)`, id, change.UserID); err != nil {
			return err
		}
	}
	return nil
}

// LiveRevocationReason lets a periodic authority failure preserve the committed
// revocation reason even if it races the process-local post-commit callback.
func (s *AccountService) LiveRevocationReason(ctx context.Context, bound LiveAccount) string {
	var reason string
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(revoked_reason,'') FROM auth_sessions WHERE id=$1 AND user_id=$2 AND revoked_at IS NOT NULL`, bound.Session.ID, bound.Session.User.ID).Scan(&reason)
	return reason
}

// Revoked closes affected process-local sockets after committed database cleanup.
func (s *AccountService) Revoked(change auth.Revocation) {
	if s.liveRevoked != nil {
		s.liveRevoked(change)
	}
	// Emit current status to surviving viewers after their sender was revoked.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM route_members WHERE user_id=$1 AND left_at IS NULL`, change.UserID)
	if err != nil {
		return
	} // Authority is committed; periodic checks still enforce it.
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return
	}
	for _, id := range ids {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return
		}
		m, err := liveMember(ctx, tx, id)
		_ = tx.Rollback(ctx)
		if err == nil && (m.Status == MemberStatusSpectating || m.Status == MemberStatusOffline) {
			kind := "member_stopped_sharing"
			if m.Status == MemberStatusOffline {
				kind = "member_went_offline"
			}
			s.publishPresence([]presenceChange{{m, kind}})
		}
	}
}

// RecoverLive clears abandoned runtime state before accepting connections.
// The caller must hold the singleton runtime lock obtained by StartLive.
func (s *AccountService) RecoverLive(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT id FROM routes WHERE owner_user_id IS NOT NULL ORDER BY id FOR UPDATE`); err != nil {
		return err
	}
	for _, sql := range []string{
		`UPDATE path_segments SET ended_at=clock_timestamp(),end_reason='disconnected' WHERE ended_at IS NULL AND route_id IN(SELECT id FROM routes WHERE owner_user_id IS NOT NULL)`,
		`DELETE FROM account_sharing_leases`, `DELETE FROM account_live_connections`,
		`UPDATE route_members SET status='offline',live_absent_since=clock_timestamp() WHERE user_id IS NOT NULL AND left_at IS NULL`,
	} {
		if _, err = tx.Exec(ctx, sql); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// SweepLive applies persisted sender deadlines under route locks. Viewer traffic never resets them.
func (s *AccountService) SweepLive(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT route_id::text FROM account_sharing_leases UNION SELECT DISTINCT route_id::text FROM account_live_connections UNION SELECT DISTINCT route_id::text FROM route_members WHERE user_id IS NOT NULL AND status='spectating' AND live_absent_since IS NOT NULL`)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = s.sweepRoute(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *AccountService) sweepRoute(ctx context.Context, routeID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT id FROM routes WHERE id=$1 FOR UPDATE`, routeID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM account_live_connections c WHERE route_id=$1 AND NOT EXISTS(SELECT 1 FROM auth_sessions se WHERE se.id=c.session_id AND se.revoked_at IS NULL AND se.expires_at>clock_timestamp())`, routeID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT l.member_id::text, l.stale_at IS NULL AND (l.last_position_at<=clock_timestamp()-$2*interval '1 second' OR NOT EXISTS(SELECT 1 FROM account_live_connections WHERE id=l.connection_id)),
        (l.stale_at IS NOT NULL AND l.stale_at<=clock_timestamp()-$3*interval '1 second') OR NOT EXISTS(SELECT 1 FROM auth_sessions se WHERE se.id=l.session_id AND se.revoked_at IS NULL AND se.expires_at>clock_timestamp())
        FROM account_sharing_leases l WHERE route_id=$1`, routeID, s.liveConfig.TrackingStaleAfter.Seconds(), s.liveConfig.TrackingOfflineAfter.Seconds())
	if err != nil {
		return err
	}
	type deadline struct {
		memberID       string
		stale, offline bool
	}
	deadlines, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (deadline, error) {
		var d deadline
		err := row.Scan(&d.memberID, &d.stale, &d.offline)
		return d, err
	})
	if err != nil {
		return err
	}
	var changes []presenceChange
	for _, d := range deadlines {
		if d.offline {
			change, err := finishLease(ctx, tx, d.memberID)
			if err != nil {
				return err
			}
			changes = append(changes, change)
		} else if d.stale {
			if _, err = tx.Exec(ctx, `UPDATE account_sharing_leases SET stale_at=clock_timestamp() WHERE member_id=$1`, d.memberID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE route_members SET status='stale' WHERE id=$1 AND status='tracking'`, d.memberID); err != nil {
				return err
			}
			m, err := liveMember(ctx, tx, d.memberID)
			if err != nil {
				return err
			}
			changes = append(changes, presenceChange{m, "member_became_stale"})
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE route_members m SET live_absent_since=COALESCE(live_absent_since,clock_timestamp()) WHERE route_id=$1 AND user_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM account_live_connections c WHERE c.member_id=m.id)`, routeID); err != nil {
		return err
	}
	rows, err = tx.Query(ctx, `UPDATE route_members m SET status='offline' WHERE route_id=$1 AND user_id IS NOT NULL AND status='spectating' AND live_absent_since<=clock_timestamp()-$2*interval '1 second' AND NOT EXISTS(SELECT 1 FROM account_live_connections c WHERE c.member_id=m.id) RETURNING id::text`, routeID, s.liveConfig.SpectatorOfflineAfter.Seconds())
	if err != nil {
		return err
	}
	offlineIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range offlineIDs {
		m, err := liveMember(ctx, tx, id)
		if err != nil {
			return err
		}
		changes = append(changes, presenceChange{m, "member_went_offline"})
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.publishPresence(changes)
	return nil
}

// StartLive fences a second API process, recovers state, and runs bounded deadline checks.
// KeepUp currently has one live hub; replicas require a distributed event transport.
func (s *AccountService) StartLive(ctx context.Context, logger *slog.Logger, onFailure func()) (func(), error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	var locked bool
	// Scope the singleton to this database/schema, including isolated test schemas.
	err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(hashtextextended(current_database()||':'||current_schema()||':keepup.account.live',0))`).Scan(&locked)
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("acquire account live runtime: %w", err)
	}
	if !locked {
		conn.Release()
		return nil, errors.New("account live runtime already active")
	}
	release := func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Close rather than return a potentially locked session to the pool.
		_ = conn.Conn().Close(closeCtx)
		conn.Release()
	}
	if err = s.RecoverLive(ctx); err != nil {
		release()
		return nil, err
	}
	s.livePID.Store(conn.Conn().PgConn().PID())
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	interval := min(time.Second, s.liveConfig.TrackingStaleAfter, s.liveConfig.TrackingOfflineAfter, s.liveConfig.SpectatorOfflineAfter)
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				// If the lock connection dies, stop accepting authority from this process.
				pingCtx, pingCancel := context.WithTimeout(runCtx, 5*time.Second)
				err := conn.Ping(pingCtx)
				pingCancel()
				if err != nil {
					if runCtx.Err() != nil {
						return
					}
					s.liveStopped.Store(true)
					logger.Error("account live runtime lock lost")
					if onFailure != nil {
						onFailure()
					}
					return
				}
				if err := s.SweepLive(runCtx); err != nil && runCtx.Err() == nil {
					logger.Error("account live deadline check failed", "error", err)
				}
			}
		}
	}()
	return func() { cancel(); <-done; release() }, nil
}
