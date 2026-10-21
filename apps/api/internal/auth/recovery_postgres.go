package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// All issuance, consumption, replacement and delivery lock the user before
// tokens/outbox. This also serializes mail quotas and concurrent password changes.
func (r *PostgresRepository) issueToken(ctx context.Context, tx pgx.Tx, userID, purpose string, recovery *recoveryRuntime) error {
	p := recovery.config.Policy
	var count int
	var cooling bool
	err := tx.QueryRow(ctx, `SELECT count(*),COALESCE(bool_or(purpose=$3 AND created_at>now()-$4*interval '1 second'),false)
        FROM auth_tokens WHERE user_id=$1 AND created_at>now()-$2*interval '1 second'`, userID, p.MailWindow.Seconds(), purpose, p.ResendCooldown.Seconds()).Scan(&count, &cooling)
	if err != nil {
		return fmt.Errorf("check mail quota: %w", err)
	}
	// Cooldown is checked separately if configured longer than the mail window.
	if !cooling {
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_tokens WHERE user_id=$1 AND purpose=$2 AND created_at>now()-$3*interval '1 second')`, userID, purpose, p.ResendCooldown.Seconds()).Scan(&cooling)
	}
	if err != nil {
		return fmt.Errorf("check mail cooldown: %w", err)
	}
	if count >= p.MailPerAccount {
		return &RateLimitError{RetryAfter: p.MailWindow}
	}
	if cooling {
		return &RateLimitError{RetryAfter: p.ResendCooldown}
	}
	if err = r.invalidatePurpose(ctx, tx, userID, purpose); err != nil {
		return err
	}
	ttl, path := p.VerificationTTL, "/auth/verify"
	if purpose == "reset" {
		ttl, path = p.ResetTTL, "/auth/reset-password"
	}
	token := newToken()
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO auth_tokens (user_id,purpose,token_hash,expires_at)
        VALUES ($1,$2,$3,now()+$4*interval '1 second') RETURNING id`, userID, purpose, tokenHash(token), ttl.Seconds()).Scan(&id)
	if err != nil {
		return fmt.Errorf("insert purpose token: %w", err)
	}
	link := recovery.config.PublicURL + path + "#token=" + token
	_, err = tx.Exec(ctx, `INSERT INTO auth_mail_outbox (token_id,payload) VALUES ($1,$2)`, id, recovery.encrypt(id, link))
	if err != nil {
		return fmt.Errorf("enqueue purpose email: %w", err)
	}
	return nil
}

func (r *PostgresRepository) invalidatePurpose(ctx context.Context, tx pgx.Tx, userID, purpose string) error {
	if _, err := tx.Exec(ctx, `UPDATE auth_tokens SET invalidated_at=now()
        WHERE user_id=$1 AND purpose=$2 AND consumed_at IS NULL AND invalidated_at IS NULL`, userID, purpose); err != nil {
		return fmt.Errorf("invalidate purpose tokens: %w", err)
	}
	return r.cancelPurposeMail(ctx, tx, userID, purpose)
}

func (r *PostgresRepository) cancelPurposeMail(ctx context.Context, tx pgx.Tx, userID, purpose string) error {
	_, err := tx.Exec(ctx, `UPDATE auth_mail_outbox o SET status='cancelled',payload=NULL,finished_at=now()
        FROM auth_tokens t WHERE o.token_id=t.id AND t.user_id=$1 AND t.purpose=$2 AND o.status='pending'`, userID, purpose)
	if err != nil {
		return fmt.Errorf("cancel purpose email: %w", err)
	}
	return nil
}

func (r *PostgresRepository) resendVerification(ctx context.Context, sessionHash string, recovery *recoveryRuntime) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin verification resend: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var userID string
	var verified bool
	err = tx.QueryRow(ctx, `SELECT u.id,u.verified_at IS NOT NULL FROM users u JOIN auth_sessions s ON s.user_id=u.id
        WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() FOR UPDATE OF u`, sessionHash).Scan(&userID, &verified)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSession
	}
	if err != nil {
		return fmt.Errorf("lock resend account: %w", err)
	}
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp())`, sessionHash).Scan(&valid); err != nil {
		return fmt.Errorf("recheck resend session: %w", err)
	}
	if !valid {
		return ErrSession
	}
	if !verified {
		if err = r.issueToken(ctx, tx, userID, "verification", recovery); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit verification resend: %w", err)
	}
	return nil
}

func (r *PostgresRepository) forgotPassword(ctx context.Context, identifier string, recovery *recoveryRuntime) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password recovery: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id string
	err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email_key=$1 OR username_key=$1 FOR UPDATE`, identifier).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock recovery account: %w", err)
	}
	if err = r.issueToken(ctx, tx, id, "reset", recovery); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery request: %w", err)
	}
	return nil
}

func (r *PostgresRepository) consumeToken(ctx context.Context, rawToken, purpose, passwordHash string, _ *recoveryRuntime) error {
	if len(rawToken) != 43 {
		return ErrInvalidToken
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin token consumption: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id FROM auth_tokens WHERE token_hash=$1 AND purpose=$2`, tokenHash(rawToken), purpose).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return fmt.Errorf("find purpose token: %w", err)
	}
	if _, err = tx.Exec(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, userID); err != nil {
		return fmt.Errorf("lock token account: %w", err)
	}
	var id string
	var invalid, expired bool
	err = tx.QueryRow(ctx, `SELECT id,consumed_at IS NOT NULL OR invalidated_at IS NOT NULL,expires_at<=clock_timestamp()
        FROM auth_tokens WHERE token_hash=$1 AND purpose=$2 FOR UPDATE`, tokenHash(rawToken), purpose).Scan(&id, &invalid, &expired)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return fmt.Errorf("lock purpose token: %w", err)
	}
	if invalid {
		return ErrInvalidToken
	}
	if expired {
		return ErrTokenExpired
	}
	if purpose == "verification" {
		_, err = tx.Exec(ctx, `UPDATE users SET verified_at=COALESCE(verified_at,now()) WHERE id=$1`, userID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, userID, passwordHash)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, userID)
		}
	}
	if err != nil {
		return fmt.Errorf("apply purpose token: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_tokens SET consumed_at=now() WHERE id=$1`, id); err != nil {
		return fmt.Errorf("consume purpose token: %w", err)
	}
	if err = r.cancelPurposeMail(ctx, tx, userID, purpose); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit token consumption: %w", err)
	}
	return nil
}

func (r *PostgresRepository) changePassword(ctx context.Context, sessionHash, oldPasswordHash, newPasswordHash, newSessionHash string, ttl time.Duration) (Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Session{}, fmt.Errorf("begin password change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var session Session
	err = tx.QueryRow(ctx, `SELECT u.id,u.email,u.username FROM users u JOIN auth_sessions s ON s.user_id=u.id
        WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() AND u.password_hash=$2 AND u.verified_at IS NOT NULL
        FOR UPDATE OF u`, sessionHash, oldPasswordHash).Scan(&session.User.ID, &session.User.Email, &session.User.Username)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("lock password change: %w", err)
	}
	// Recheck session authority after waiting for the user lock. Reset/rotation
	// can have changed it while the lock query was waiting.
	err = tx.QueryRow(ctx, `UPDATE auth_sessions SET token_hash=$2,last_used_at=now(),expires_at=now()+$3*interval '1 second'
        WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>clock_timestamp() RETURNING id,expires_at`, sessionHash, newSessionHash, ttl.Seconds()).Scan(&session.ID, &session.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("rotate changed session: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, session.User.ID, newPasswordHash); err != nil {
		return Session{}, fmt.Errorf("update password: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=now() WHERE user_id=$1 AND id<>$2 AND revoked_at IS NULL`, session.User.ID, session.ID); err != nil {
		return Session{}, fmt.Errorf("revoke other sessions: %w", err)
	}
	if err = r.invalidatePurpose(ctx, tx, session.User.ID, "reset"); err != nil {
		return Session{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, fmt.Errorf("commit password change: %w", err)
	}
	session.User.Verified = true
	return session, nil
}
