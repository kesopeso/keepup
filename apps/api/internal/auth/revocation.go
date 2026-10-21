package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Revocation identifies sessions affected by a committed authority change.
// Empty SessionID means all sessions except ExceptSessionID, when supplied.
type Revocation struct {
	UserID, SessionID, ExceptSessionID, Reason string
	SessionIDs                                 []string
}

// Matches reports whether a connection belongs to this revocation.
func (r Revocation) Matches(userID, sessionID string) bool {
	if r.SessionIDs != nil {
		if r.UserID != userID {
			return false
		}
		for _, id := range r.SessionIDs {
			if id == sessionID {
				return true
			}
		}
		return false
	}
	return r.UserID == userID && (r.SessionID == "" || r.SessionID == sessionID) && r.ExceptSessionID != sessionID
}

// SessionRevoker releases persistent sharing inside the authority transaction,
// then disconnects local sockets after commit and before success is returned.
type SessionRevoker interface {
	Revoke(context.Context, pgx.Tx, *Revocation) error
	Revoked(Revocation)
}

// WithSessionRevoker installs account live cleanup before serving requests.
func WithSessionRevoker(hook SessionRevoker) ServiceOption {
	return func(s *Service) error {
		if hook == nil {
			return errors.New("session revoker is required")
		}
		s.repo.revoker = hook
		return nil
	}
}

func (r *PostgresRepository) revokeLive(ctx context.Context, tx pgx.Tx, change *Revocation) error {
	if r.revoker != nil {
		return r.revoker.Revoke(ctx, tx, change)
	}
	return nil
}

func (r *PostgresRepository) revoked(change Revocation) {
	if r.revoker != nil {
		r.revoker.Revoked(change)
	}
}
