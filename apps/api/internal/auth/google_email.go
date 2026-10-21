package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func googleIdentityError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrIdentityConflict
	}
	return fmt.Errorf("google identity persistence: %w", err)
}

// checkMailQuota serializes by normalized recipient, including flows without a user.
// User or flow locks precede this quota lock; no caller acquires either afterwards.
func (r *PostgresRepository) checkMailQuota(ctx context.Context, tx pgx.Tx, emailKey, purpose string, recovery *recoveryRuntime) error {
	if err := accountKeyLock(ctx, tx, "mail:recipient:"+emailKey); err != nil {
		return err
	}
	p := recovery.config.Policy
	var count int
	var cooling bool
	err := tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE t.created_at>clock_timestamp()-$2*interval '1 second'),
        COALESCE(bool_or(t.purpose=$3 AND t.created_at>clock_timestamp()-$4*interval '1 second'),false)
        FROM auth_tokens t LEFT JOIN users u ON u.id=t.user_id LEFT JOIN auth_google_flows f ON f.id=t.google_flow_id
        WHERE COALESCE(u.email_key,f.email_key)=$1
        AND t.created_at>clock_timestamp()-GREATEST($2,$4)*interval '1 second'`, emailKey, p.MailWindow.Seconds(), purpose, p.ResendCooldown.Seconds()).Scan(&count, &cooling)
	if err != nil {
		return fmt.Errorf("check recipient quota: %w", err)
	}
	if count >= p.MailPerAccount {
		return &RateLimitError{RetryAfter: p.MailWindow}
	}
	if cooling {
		return &RateLimitError{RetryAfter: p.ResendCooldown}
	}
	return nil
}

func (r *PostgresRepository) issueGoogleEmail(ctx context.Context, tx pgx.Tx, f googleFlow, recovery *recoveryRuntime) error {
	if err := r.checkMailQuota(ctx, tx, NormalizeIdentifier(f.Email), "google_email", recovery); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE auth_tokens SET invalidated_at=clock_timestamp()
        WHERE google_flow_id=$1 AND consumed_at IS NULL AND invalidated_at IS NULL`, f.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE auth_mail_outbox o SET status='cancelled',payload=NULL,finished_at=clock_timestamp()
        FROM auth_tokens t WHERE t.id=o.token_id AND t.google_flow_id=$1 AND o.status='pending'`, f.ID); err != nil {
		return err
	}
	token := newToken()
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO auth_tokens (google_flow_id,purpose,token_hash,expires_at)
        VALUES ($1,'google_email',$2,LEAST($3,clock_timestamp()+$4*interval '1 second')) RETURNING id`, f.ID, tokenHash(token), f.ExpiresAt, recovery.config.Policy.VerificationTTL.Seconds()).Scan(&id)
	if err != nil {
		return err
	}
	link := recovery.config.PublicURL + "/auth/google/verify#token=" + token
	_, err = tx.Exec(ctx, `INSERT INTO auth_mail_outbox (token_id,payload) VALUES ($1,$2)`, id, recovery.encrypt(id, link))
	return err
}

func (r *PostgresRepository) confirmGoogleEmail(ctx context.Context, token string) error {
	if len(token) != 43 {
		return ErrInvalidToken
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var flowID string
	err = tx.QueryRow(ctx, `SELECT google_flow_id FROM auth_tokens WHERE token_hash=$1 AND purpose='google_email'`, tokenHash(token)).Scan(&flowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}
	var status string
	var expired bool
	err = tx.QueryRow(ctx, `SELECT status,expires_at<=clock_timestamp() FROM auth_google_flows WHERE id=$1 FOR UPDATE`, flowID).Scan(&status, &expired)
	if err != nil {
		return err
	}
	if status != "email_verification_required" {
		return ErrInvalidToken
	}
	if expired {
		return ErrTokenExpired
	}
	var id string
	var invalid bool
	err = tx.QueryRow(ctx, `SELECT id,consumed_at IS NOT NULL OR invalidated_at IS NOT NULL,expires_at<=clock_timestamp()
        FROM auth_tokens WHERE token_hash=$1 AND purpose='google_email' FOR UPDATE`, tokenHash(token)).Scan(&id, &invalid, &expired)
	if err != nil {
		return err
	}
	if invalid {
		return ErrInvalidToken
	}
	if expired {
		return ErrTokenExpired
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_tokens SET consumed_at=clock_timestamp() WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_mail_outbox SET status='cancelled',payload=NULL,finished_at=clock_timestamp() WHERE token_id=$1 AND status='pending'`, id); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_google_flows SET status='ready' WHERE id=$1`, flowID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
