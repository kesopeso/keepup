package routes

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"keepup/apps/api/internal/auth"
	"keepup/apps/api/internal/config"
)

// AccountService binds route operations to verified, revocable account sessions.
type AccountService struct {
	pool                       *pgxpool.Pool
	maxTracking                int
	position                   config.PositionValidationConfig
	idle, ticketTTL, retention time.Duration
	cursorKey                  []byte
}

// NewAccountService shares lifecycle persistence but never accepts legacy route tokens.
func NewAccountService(pool *pgxpool.Pool, maxTracking int, position config.PositionValidationConfig, idle, ticketTTL, retention time.Duration, key []byte) (*AccountService, error) {
	if pool == nil || maxTracking < 1 || idle <= 0 || ticketTTL <= 0 || retention <= 0 || len(key) != 32 {
		return nil, fmt.Errorf("invalid account route configuration")
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("keepup.routes.cursor.v1"))
	return &AccountService{pool: pool, maxTracking: maxTracking, position: position, idle: idle, ticketTTL: ticketTTL, retention: retention, cursorKey: mac.Sum(nil)}, nil
}

type accountRoute struct {
	AuthorizedMember
	ownerID      string
	passwordHash string
}

func scanAccountRoute(row pgx.Row) (accountRoute, error) {
	var a accountRoute
	err := row.Scan(&a.Route.ID, &a.Route.Code, &a.Route.Name, &a.Route.Description, &a.Route.HasPassword, &a.Route.SharingPolicy, &a.Route.Status, &a.Route.MaxTrackingMembers, &a.Route.CreatedAt, &a.Route.ClosedAt, &a.ownerID, &a.passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrRouteNotFound
	}
	return a, err
}

func loadAccountRoute(ctx context.Context, tx pgx.Tx, code string) (accountRoute, error) {
	return scanAccountRoute(tx.QueryRow(ctx, `SELECT id,code,name,COALESCE(description,''),password_hash IS NOT NULL,sharing_policy,status,max_tracking_members,created_at,closed_at,
        owner_user_id,COALESCE(password_hash,'') FROM routes WHERE code=$1 AND owner_user_id IS NOT NULL FOR UPDATE`, normalizeCode(code)))
}

func loadAccountMember(ctx context.Context, tx pgx.Tx, a *accountRoute, userID string, current bool) error {
	query := `SELECT m.id,m.route_id,u.id,u.username,m.transport_mode,m.is_owner,m.status,m.color,m.joined_at,m.left_at
        FROM route_members m JOIN users u ON u.id=m.user_id WHERE m.route_id=$1 AND m.user_id=$2`
	if current {
		query += ` AND m.left_at IS NULL AND m.status<>'left'`
	}
	query += ` ORDER BY m.joined_at DESC,m.id DESC LIMIT 1`
	m := &a.Member
	err := tx.QueryRow(ctx, query, a.Route.ID, userID).Scan(&m.ID, &m.RouteID, &m.UserID, &m.DisplayName, &m.TransportMode, &m.IsOwner, &m.Status, &m.Color, &m.JoinedAt, &m.LeftAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUnauthorized
	}
	if err == nil {
		m.IsOwner = a.ownerID == userID
	}
	return err
}

func (s *AccountService) lockSession(ctx context.Context, tx pgx.Tx, session auth.Session) (auth.User, error) {
	var user auth.User
	err := tx.QueryRow(ctx, `SELECT id,email,username,verified_at IS NOT NULL FROM users WHERE id=$1 FOR UPDATE`, session.User.ID).Scan(&user.ID, &user.Email, &user.Username, &user.Verified)
	if errors.Is(err, pgx.ErrNoRows) {
		return user, auth.ErrSession
	}
	if err != nil {
		return user, err
	}
	var valid bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_sessions WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL AND expires_at>clock_timestamp())`, session.ID, user.ID).Scan(&valid)
	if err != nil {
		return user, err
	}
	if !valid {
		return user, auth.ErrSession
	}
	if !user.Verified {
		return user, auth.ErrPending
	}
	_, err = tx.Exec(ctx, `UPDATE auth_sessions SET last_used_at=clock_timestamp(),expires_at=clock_timestamp()+$2*interval '1 second' WHERE id=$1`, session.ID, s.idle.Seconds())
	return user, err
}

// User -> route -> membership locks coordinate with account recovery and lifecycle writes.
func accountOperation[T any](ctx context.Context, s *AccountService, session auth.Session, code string, current, owner bool, run func(pgx.Tx, *Service, accountRoute, auth.User) (T, error)) (T, error) {
	var zero T
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	user, err := s.lockSession(ctx, tx, session)
	if err != nil {
		return zero, err
	}
	var a accountRoute
	if code != "" {
		a, err = loadAccountRoute(ctx, tx, code)
		if err != nil {
			return zero, err
		}
		if owner && a.ownerID != user.ID {
			return zero, ErrUnauthorized
		}
		if current || owner {
			if err = loadAccountMember(ctx, tx, &a, user.ID, current); err != nil {
				return zero, err
			}
		}
	}
	repo := &accountRepository{PostgresRepository: &PostgresRepository{db: routeTransaction{tx}}, authorized: a.AuthorizedMember}
	core := NewService(repo, s.maxTracking, s.position)
	value, err := run(tx, core, a, user)
	if err != nil {
		return zero, err
	}
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT revoked_at IS NULL AND expires_at>clock_timestamp() FROM auth_sessions WHERE id=$1 AND user_id=$2`, session.ID, user.ID).Scan(&valid); err != nil {
		return zero, err
	}
	if !valid {
		return zero, auth.ErrSession
	}
	if err = tx.Commit(ctx); err != nil {
		return zero, err
	}
	return value, nil
}

type accountRepository struct {
	*PostgresRepository
	authorized AuthorizedMember
}

func (r *accountRepository) GetAuthorizedMemberByTokenHash(context.Context, string) (AuthorizedMember, error) {
	return r.authorized, nil
}
func (r *accountRepository) GetAuthorizedOwnerByTokenHash(context.Context, string) (AuthorizedMember, error) {
	if !r.authorized.Member.IsOwner {
		return AuthorizedMember{}, ErrUnauthorized
	}
	return r.authorized, nil
}
func (r *accountRepository) GetMembersByRouteID(ctx context.Context, id string) ([]Member, error) {
	rows, err := r.db.Query(ctx, `SELECT m.id,m.route_id,u.id,u.username,m.transport_mode,m.is_owner,m.status,m.color,m.joined_at,m.left_at
        FROM route_members m JOIN users u ON u.id=m.user_id WHERE m.route_id=$1 ORDER BY m.joined_at,m.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []Member{}
	for rows.Next() {
		var m Member
		if err = rows.Scan(&m.ID, &m.RouteID, &m.UserID, &m.DisplayName, &m.TransportMode, &m.IsOwner, &m.Status, &m.Color, &m.JoinedAt, &m.LeftAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func accountMember(m Member, user auth.User) Member {
	m.ClientID = ""
	m.UserID = user.ID
	m.DisplayName = user.Username
	return m
}

// CreateRoute creates an owner membership without independent owner/member credentials.
func (s *AccountService) CreateRoute(ctx context.Context, session auth.Session, input CreateRouteInput) (CreateRouteResult, error) {
	if input.ClientID != "" || input.DisplayName != "" {
		return CreateRouteResult{}, ErrInvalidInput
	}
	return accountOperation(ctx, s, session, "", false, false, func(tx pgx.Tx, _ *Service, _ accountRoute, user auth.User) (CreateRouteResult, error) {
		input.ClientID, input.DisplayName = user.ID, user.Username
		normalized, err := normalizeCreateInput(input)
		if err != nil {
			return CreateRouteResult{}, err
		}
		hash, err := hashPassword(normalized.Password)
		if err != nil {
			return CreateRouteResult{}, err
		}
		code, err := newRouteCode(defaultCodeLength)
		if err != nil {
			return CreateRouteResult{}, err
		}
		// Each code insertion uses a savepoint so a rare unique collision is retryable.
		var a accountRoute
		for attempt := 0; attempt < maxCodeAttempts; attempt++ {
			nested, beginErr := tx.Begin(ctx)
			if beginErr != nil {
				return CreateRouteResult{}, beginErr
			}
			a, err = scanAccountRoute(nested.QueryRow(ctx, `INSERT INTO routes (code,name,description,password_hash,sharing_policy,status,max_tracking_members,owner_user_id)
                VALUES ($1,$2,$3,NULLIF($4,''),$5,'active',$6,$7) RETURNING id,code,name,COALESCE(description,''),password_hash IS NOT NULL,sharing_policy,status,max_tracking_members,created_at,closed_at,owner_user_id,COALESCE(password_hash,'')`, code, normalized.Name, normalized.Description, hash, normalized.SharingPolicy, s.maxTracking, user.ID))
			if err == nil {
				err = nested.Commit(ctx)
				break
			}
			_ = nested.Rollback(ctx)
			if !isCodeConflict(err) {
				return CreateRouteResult{}, err
			}
			code, err = newRouteCode(defaultCodeLength)
			if err != nil {
				return CreateRouteResult{}, err
			}
		}
		if err != nil {
			return CreateRouteResult{}, err
		}
		if a.Route.ID == "" {
			return CreateRouteResult{}, fmt.Errorf("exhausted route code attempts")
		}
		member, err := insertAccountMember(ctx, tx, a.Route.ID, user, normalized.TransportMode, true, palette[0])
		if err != nil {
			return CreateRouteResult{}, err
		}
		return CreateRouteResult{Route: a.Route, Owner: member}, nil
	})
}

func insertAccountMember(ctx context.Context, tx pgx.Tx, routeID string, user auth.User, mode string, owner bool, color string) (Member, error) {
	var m Member
	err := tx.QueryRow(ctx, `INSERT INTO route_members(route_id,user_id,client_id,display_name,transport_mode,is_owner,status,color)
        VALUES ($1,$2::uuid,$2::text,$3,$4,$5,'spectating',$6) RETURNING id,route_id,transport_mode,is_owner,status,color,joined_at,left_at`, routeID, user.ID, user.Username, mode, owner, color).Scan(&m.ID, &m.RouteID, &m.TransportMode, &m.IsOwner, &m.Status, &m.Color, &m.JoinedAt, &m.LeftAt)
	return accountMember(m, user), err
}

// JoinRoute restores a current episode, or explicitly creates a new active/archive episode.
func (s *AccountService) JoinRoute(ctx context.Context, session auth.Session, code string, input JoinRouteInput) (JoinRouteResult, error) {
	if input.ClientID != "" || input.DisplayName != "" {
		return JoinRouteResult{}, ErrInvalidInput
	}
	return accountOperation(ctx, s, session, code, false, false, func(tx pgx.Tx, _ *Service, a accountRoute, user auth.User) (JoinRouteResult, error) {
		err := loadAccountMember(ctx, tx, &a, user.ID, true)
		if err == nil {
			return JoinRouteResult{Route: a.Route, Member: a.Member, Restored: true}, nil
		}
		if !errors.Is(err, ErrUnauthorized) {
			return JoinRouteResult{}, err
		}
		input.ClientID, input.DisplayName = user.ID, user.Username
		normalized, err := normalizeJoinInput(input)
		if err != nil {
			return JoinRouteResult{}, err
		}
		if a.Route.HasPassword && verifyPassword(a.passwordHash, normalized.Password) != nil {
			return JoinRouteResult{}, ErrInvalidPassword
		}
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM route_members WHERE route_id=$1`, a.Route.ID).Scan(&count); err != nil {
			return JoinRouteResult{}, err
		}
		m, err := insertAccountMember(ctx, tx, a.Route.ID, user, normalized.TransportMode, a.ownerID == user.ID, palette[(count+1)%len(palette)])
		return JoinRouteResult{Route: a.Route, Member: m}, err
	})
}

// AccessRoute reveals join metadata only to an active account, not route history.
func (s *AccountService) AccessRoute(ctx context.Context, session auth.Session, code string) (AccessRouteResult, error) {
	return accountOperation(ctx, s, session, code, false, false, func(_ pgx.Tx, _ *Service, a accountRoute, _ auth.User) (AccessRouteResult, error) {
		r := a.Route
		return AccessRouteResult{Code: r.Code, Name: r.Name, Description: r.Description, Status: r.Status, RequiresPassword: r.HasPassword, SharingPolicy: r.SharingPolicy}, nil
	})
}

// Snapshot requires a current membership, including for archived routes.
func (s *AccountService) Snapshot(ctx context.Context, session auth.Session, code string) (Snapshot, error) {
	return accountOperation(ctx, s, session, code, true, false, func(_ pgx.Tx, core *Service, _ accountRoute, _ auth.User) (Snapshot, error) {
		return core.Snapshot(ctx, code, "account-authority")
	})
}

// UpdateRoute checks account ownership in the same transaction as the lifecycle change.
func (s *AccountService) UpdateRoute(ctx context.Context, session auth.Session, code string, input UpdateRouteInput) (Route, error) {
	return accountOperation(ctx, s, session, code, false, true, func(_ pgx.Tx, core *Service, _ accountRoute, _ auth.User) (Route, error) {
		return core.UpdateRoute(ctx, code, "account-authority", input)
	})
}

// LeaveRoute preserves the historical membership episode and its path records.
func (s *AccountService) LeaveRoute(ctx context.Context, session auth.Session, code string) (LeaveRouteResult, error) {
	return accountOperation(ctx, s, session, code, true, false, func(_ pgx.Tx, core *Service, _ accountRoute, user auth.User) (LeaveRouteResult, error) {
		result, err := core.LeaveRoute(ctx, code, "account-authority")
		result.Member = accountMember(result.Member, user)
		return result, err
	})
}

// DeleteRoute removes only a route owned by the authenticated account.
func (s *AccountService) DeleteRoute(ctx context.Context, session auth.Session, code string) error {
	_, err := accountOperation(ctx, s, session, code, false, true, func(_ pgx.Tx, core *Service, _ accountRoute, _ auth.User) (struct{}, error) {
		return struct{}{}, core.DeleteRoute(ctx, code, "account-authority")
	})
	return err
}
