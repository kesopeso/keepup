package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository persists account/session authority. Migrations run separately.
type PostgresRepository struct{ pool *pgxpool.Pool }

// NewPostgresRepository uses the application's existing database pool.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func uniqueError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_email_key_key":
			return ErrEmailTaken
		case "users_username_key_key":
			return ErrUsernameTaken
		}
	}
	return fmt.Errorf("account persistence: %w", err)
}

func (r *PostgresRepository) signup(ctx context.Context, input SignupInput, hash, credentialHash string, ttl time.Duration, recovery *recoveryRuntime) (Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("begin signup: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = accountKeyLock(ctx, tx, "account:email:"+NormalizeIdentifier(input.Email)); err != nil {
		return Session{}, err
	}
	var session Session
	err = tx.QueryRow(ctx, `INSERT INTO users (email,email_key,username,username_key,password_hash)
		VALUES ($1,$2,$3,$4,$5) RETURNING id,email,username,verified_at IS NOT NULL`,
		input.Email, NormalizeIdentifier(input.Email), input.Username, NormalizeIdentifier(input.Username), hash,
	).Scan(&session.User.ID, &session.User.Email, &session.User.Username, &session.User.Verified)
	if err != nil {
		return Session{}, uniqueError(err)
	}
	err = tx.QueryRow(ctx, `INSERT INTO auth_sessions (user_id,token_hash,last_used_at,expires_at)
		VALUES ($1,$2,now(),now()+$3*interval '1 second') RETURNING id,expires_at`,
		session.User.ID, credentialHash, ttl.Seconds()).Scan(&session.ID, &session.ExpiresAt)
	if err != nil {
		return Session{}, fmt.Errorf("insert signup session: %w", err)
	}
	if err = r.issueToken(ctx, tx, session.User.ID, "verification", recovery); err != nil {
		return Session{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit signup: %w", err)
	}
	return session, nil
}

func (r *PostgresRepository) password(ctx context.Context, identifier string) (string, string, error) {
	var id, hash string
	err := r.pool.QueryRow(ctx, `SELECT id,password_hash FROM users WHERE email_key=$1 OR username_key=$1`, identifier).Scan(&id, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrCredentials
	}
	if err != nil {
		return "", "", fmt.Errorf("find credentials: %w", err)
	}
	return id, hash, nil
}

func (r *PostgresRepository) login(ctx context.Context, id, passwordHash, credentialHash string, ttl time.Duration) (Session, error) {
	var session Session
	// Lock the user before session creation. Password change/reset must take the
	// same lock before changing the hash or revoking sessions in B3.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("begin login: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `SELECT id,email,username,verified_at IS NOT NULL FROM users
		WHERE id=$1 AND password_hash=$2 FOR UPDATE`, id, passwordHash).Scan(&session.User.ID, &session.User.Email, &session.User.Username, &session.User.Verified)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrCredentials
	}
	if err != nil {
		return Session{}, fmt.Errorf("lock login account: %w", err)
	}
	err = tx.QueryRow(ctx, `INSERT INTO auth_sessions (user_id,token_hash,last_used_at,expires_at)
		VALUES ($1,$2,now(),now()+$3*interval '1 second') RETURNING id,expires_at`, id, credentialHash, ttl.Seconds()).Scan(&session.ID, &session.ExpiresAt)
	if err != nil {
		return Session{}, fmt.Errorf("insert login session: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit login: %w", err)
	}
	return session, nil
}

func (r *PostgresRepository) authenticate(ctx context.Context, hash string, ttl time.Duration) (Session, error) {
	var session Session
	err := r.pool.QueryRow(ctx, `UPDATE auth_sessions s SET last_used_at=now(),expires_at=now()+$2*interval '1 second'
		FROM users u WHERE s.token_hash=$1 AND s.user_id=u.id AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp()
		RETURNING s.id,s.expires_at,u.id,u.email,u.username,u.verified_at IS NOT NULL`, hash, ttl.Seconds(),
	).Scan(&session.ID, &session.ExpiresAt, &session.User.ID, &session.User.Email, &session.User.Username, &session.User.Verified)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("authenticate session: %w", err)
	}
	return session, nil
}

func (r *PostgresRepository) logout(ctx context.Context, hash string, everywhere bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin logout: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var userID, sessionID string
	// Use the same user lock as login. Concurrent login either precedes this
	// revocation and is revoked, or follows it as a fresh authenticated login.
	err = tx.QueryRow(ctx, `SELECT u.id,s.id FROM users u JOIN auth_sessions s ON s.user_id=u.id
		WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() FOR UPDATE OF u`, hash).Scan(&userID, &sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSession
	}
	if err != nil {
		return fmt.Errorf("lock logout account: %w", err)
	}
	_, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE user_id=$1
		AND revoked_at IS NULL AND ($2 OR id=$3)`, userID, everywhere, sessionID)
	if err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit logout: %w", err)
	}
	return nil
}

func (r *PostgresRepository) allowAttempt(ctx context.Context, key string, window time.Duration, limit int) error {
	var count int
	err := r.pool.QueryRow(ctx, `INSERT INTO auth_attempts (key_hash,window_started_at,attempts) VALUES ($1,now(),1)
		ON CONFLICT (key_hash) DO UPDATE SET
		attempts=CASE WHEN auth_attempts.window_started_at <= now()-$2*interval '1 second' THEN 1 ELSE auth_attempts.attempts+1 END,
		window_started_at=CASE WHEN auth_attempts.window_started_at <= now()-$2*interval '1 second' THEN now() ELSE auth_attempts.window_started_at END
		RETURNING attempts`, tokenHash(key), window.Seconds()).Scan(&count)
	if err != nil {
		return fmt.Errorf("count authentication attempt: %w", err)
	}
	if count > limit {
		return &RateLimitError{RetryAfter: window}
	}
	return nil
}

func (r *PostgresRepository) rotate(ctx context.Context, oldHash, newHash string, ttl time.Duration) (Session, error) {
	var session Session
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("begin rotation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	err = tx.QueryRow(ctx, `SELECT u.id FROM users u JOIN auth_sessions s ON s.user_id=u.id
		WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() FOR UPDATE OF u`, oldHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("lock rotation account: %w", err)
	}
	err = tx.QueryRow(ctx, `UPDATE auth_sessions s SET token_hash=$2,last_used_at=now(),expires_at=now()+$3*interval '1 second'
		FROM users u WHERE s.user_id=u.id AND s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp()
		RETURNING s.id,s.expires_at,u.id,u.email,u.username,u.verified_at IS NOT NULL`, oldHash, newHash, ttl.Seconds()).Scan(
		&session.ID, &session.ExpiresAt, &session.User.ID, &session.User.Email, &session.User.Username, &session.User.Verified)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("rotate session: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit rotation: %w", err)
	}
	return session, nil
}
