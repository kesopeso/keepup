package routes

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"keepup/apps/api/internal/auth"
)

// SocketTicket is a short-lived, single-use bootstrap credential, never a URL parameter.
type SocketTicket struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// LiveAccount binds an established connection to its session and exact membership episode.
type LiveAccount struct {
	Session           auth.Session
	Code              string
	MemberID          string
	ConnectionID      string
	SharingGeneration string
	PresenceRestored  bool
}

// IssueSocketTicket rechecks active session/current membership and refuses archives.
func (s *AccountService) IssueSocketTicket(ctx context.Context, session auth.Session, code string) (SocketTicket, error) {
	return accountOperation(ctx, s, session, code, true, false, func(tx pgx.Tx, _ *Service, a accountRoute, _ auth.User) (SocketTicket, error) {
		if a.Route.Status != RouteStatusActive {
			return SocketTicket{}, ErrRouteClosed
		}
		bytes := make([]byte, 32)
		_, _ = rand.Read(bytes)
		result := SocketTicket{Ticket: base64.RawURLEncoding.EncodeToString(bytes)}
		err := tx.QueryRow(ctx, `INSERT INTO auth_socket_tickets(token_hash,session_id,route_id,member_id,expires_at)
            VALUES ($1,$2,$3,$4,clock_timestamp()+$5*interval '1 second') RETURNING expires_at`, tokenHash(result.Ticket), session.ID, a.Route.ID, a.Member.ID, s.ticketTTL.Seconds()).Scan(&result.ExpiresAt)
		return result, err
	})
}

// ConsumeSocketTicket validates session/membership again and atomically consumes once.
func (s *AccountService) ConsumeSocketTicket(ctx context.Context, raw string) (LiveAccount, error) {
	if len(raw) != 43 {
		return LiveAccount{}, ErrUnauthorized
	}
	var bound LiveAccount
	err := s.pool.QueryRow(ctx, `SELECT t.session_id,se.user_id,r.code,t.member_id FROM auth_socket_tickets t JOIN auth_sessions se ON se.id=t.session_id
        JOIN routes r ON r.id=t.route_id WHERE t.token_hash=$1`, tokenHash(raw)).Scan(&bound.Session.ID, &bound.Session.User.ID, &bound.Code, &bound.MemberID)
	if errors.Is(err, pgx.ErrNoRows) {
		return bound, ErrUnauthorized
	}
	if err != nil {
		return bound, err
	}
	return accountOperation(ctx, s, bound.Session, bound.Code, true, false, func(tx pgx.Tx, _ *Service, a accountRoute, _ auth.User) (LiveAccount, error) {
		if a.Route.Status != RouteStatusActive {
			return LiveAccount{}, ErrRouteClosed
		}
		if a.Member.ID != bound.MemberID {
			return LiveAccount{}, ErrUnauthorized
		}
		tag, err := tx.Exec(ctx, `UPDATE auth_socket_tickets SET consumed_at=clock_timestamp() WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>clock_timestamp()`, tokenHash(raw))
		if err != nil {
			return LiveAccount{}, err
		}
		if tag.RowsAffected() != 1 {
			return LiveAccount{}, ErrUnauthorized
		}
		return bound, nil
	})
}

// AuthorizeLive rechecks account/session/current membership before delivering live data.
func (s *AccountService) AuthorizeLive(ctx context.Context, bound LiveAccount) (AuthorizedMember, error) {
	return accountOperation(ctx, s, bound.Session, bound.Code, true, false, func(tx pgx.Tx, _ *Service, a accountRoute, _ auth.User) (AuthorizedMember, error) {
		if a.Route.Status != RouteStatusActive {
			return AuthorizedMember{}, ErrRouteClosed
		}
		if a.Member.ID != bound.MemberID {
			return AuthorizedMember{}, ErrUnauthorized
		}
		if bound.ConnectionID != "" {
			if err := liveConnection(ctx, tx, bound); err != nil {
				return AuthorizedMember{}, err
			}
		}
		return a.AuthorizedMember, nil
	})
}

// MarkOnline restores an offline current episode without acquiring a sharing slot.
func (s *AccountService) MarkOnline(ctx context.Context, bound LiveAccount) (Member, bool, error) {
	type onlineResult struct {
		Member  Member
		Changed bool
	}
	result, err := accountOperation(ctx, s, bound.Session, bound.Code, true, false, func(_ pgx.Tx, core *Service, a accountRoute, user auth.User) (onlineResult, error) {
		if a.Member.ID != bound.MemberID || a.Route.Status != RouteStatusActive {
			return onlineResult{}, ErrUnauthorized
		}
		m, changed, err := core.MarkMemberOnline(ctx, a.Route.ID, a.Member.ID)
		return onlineResult{accountMember(m, user), changed}, err
	})
	return result.Member, result.Changed, err
}

// AccountSharingResult returns the generation only to the acquiring connection.
type AccountSharingResult struct {
	StartSharingResult
	SharingGeneration string
	Changed           bool
}

// StartSharing acquires one account/route lease. Retrying on its connection is idempotent.
func (s *AccountService) StartSharing(ctx context.Context, bound LiveAccount) (AccountSharingResult, error) {
	return accountOperation(ctx, s, bound.Session, bound.Code, true, false, func(tx pgx.Tx, core *Service, a accountRoute, user auth.User) (AccountSharingResult, error) {
		if a.Member.ID != bound.MemberID {
			return AccountSharingResult{}, ErrUnauthorized
		}
		if a.Route.Status != RouteStatusActive {
			return AccountSharingResult{}, ErrRouteClosed
		}
		if err := liveConnection(ctx, tx, bound); err != nil {
			return AccountSharingResult{}, err
		}
		var connection, generation string
		err := tx.QueryRow(ctx, `SELECT connection_id,generation FROM account_sharing_leases WHERE route_id=$1 AND user_id=$2`, a.Route.ID, user.ID).Scan(&connection, &generation)
		if err == nil {
			if connection != bound.ConnectionID {
				return AccountSharingResult{}, ErrSharingOnAnotherDevice
			}
			bound.SharingGeneration = generation
			if err = s.senderLease(ctx, tx, bound); err != nil {
				return AccountSharingResult{}, err
			}
			return AccountSharingResult{StartSharingResult: StartSharingResult{Member: a.Member}, SharingGeneration: generation}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return AccountSharingResult{}, err
		}
		result, err := core.StartSharing(ctx, bound.Code, "account-authority")
		result.Member = accountMember(result.Member, user)
		if err != nil {
			return AccountSharingResult{}, err
		}
		err = tx.QueryRow(ctx, `INSERT INTO account_sharing_leases(route_id,user_id,member_id,session_id,connection_id) VALUES($1,$2,$3,$4,$5) RETURNING generation`, a.Route.ID, user.ID, a.Member.ID, bound.Session.ID, bound.ConnectionID).Scan(&generation)
		return AccountSharingResult{StartSharingResult: result, SharingGeneration: generation, Changed: true}, err
	})
}

// StopSharing accepts only the owning connection and current generation.
func (s *AccountService) StopSharing(ctx context.Context, bound LiveAccount) (StopSharingResult, error) {
	return accountOperation(ctx, s, bound.Session, bound.Code, true, false, func(tx pgx.Tx, core *Service, a accountRoute, user auth.User) (StopSharingResult, error) {
		if a.Member.ID != bound.MemberID {
			return StopSharingResult{}, ErrUnauthorized
		}
		if err := s.senderLease(ctx, tx, bound); err != nil {
			return StopSharingResult{}, err
		}
		result, err := core.StopSharing(ctx, bound.Code, "account-authority")
		result.Member = accountMember(result.Member, user)
		if err == nil {
			_, err = tx.Exec(ctx, `DELETE FROM account_sharing_leases WHERE member_id=$1`, bound.MemberID)
		}
		return result, err
	})
}

// RecordPosition preserves GPS validation and scopes writes to the bound episode/session.
func (s *AccountService) RecordPosition(ctx context.Context, bound LiveAccount, input PositionUpdateInput) (PositionUpdateResult, error) {
	return accountOperation(ctx, s, bound.Session, bound.Code, true, false, func(tx pgx.Tx, core *Service, a accountRoute, user auth.User) (PositionUpdateResult, error) {
		if a.Member.ID != bound.MemberID {
			return PositionUpdateResult{}, ErrUnauthorized
		}
		if err := s.senderLease(ctx, tx, bound); err != nil {
			return PositionUpdateResult{}, err
		}
		result, err := core.RecordPosition(ctx, "account-authority", input)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE account_sharing_leases SET last_position_at=clock_timestamp(),stale_at=NULL WHERE member_id=$1`, bound.MemberID)
		}
		if result.RecoveredMember != nil {
			m := accountMember(*result.RecoveredMember, user)
			result.RecoveredMember = &m
		}
		return result, err
	})
}

// CleanupTickets retains expired bootstrap records for the account retention period.
func (s *AccountService) CleanupTickets(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_socket_tickets WHERE expires_at<clock_timestamp()-$1*interval '1 second'`, s.retention.Seconds())
	return err
}
