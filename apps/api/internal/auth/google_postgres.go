package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type googleQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func loadGoogleFlow(ctx context.Context, q googleQuerier, token, transport string, lock bool) (googleFlow, error) {
	var f googleFlow
	if len(token) != 43 {
		return f, ErrInvalidFlow
	}
	query := `SELECT id,nonce_hash,status,COALESCE(subject,''),COALESCE(email,''),audiences,expires_at FROM auth_google_flows
        WHERE token_hash=$1 AND transport=$2`
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRow(ctx, query, tokenHash(token), transport).Scan(&f.ID, &f.NonceHash, &f.Status, &f.Subject, &f.Email, &f.Audiences, &f.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return f, ErrInvalidFlow
	}
	if err != nil {
		return f, fmt.Errorf("load Google context: %w", err)
	}
	return f, flowValidity(f.Status, f.ExpiresAt)
}

func (r *PostgresRepository) readGoogleFlow(ctx context.Context, token, transport string) (googleFlow, error) {
	f, err := loadGoogleFlow(ctx, r.pool, token, transport, false)
	if err != nil {
		return f, err
	}
	if f.Status == "onboarding" {
		// Identifiers on pending password registrations remain immutable.
		err = r.pool.QueryRow(ctx, `SELECT username FROM users WHERE email_key=$1 AND verified_at IS NULL`, NormalizeIdentifier(f.Email)).Scan(&f.Username)
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
	}
	return f, err
}

// accountKeyLock also coordinates password signup when the email row does not yet exist.
// Hash collisions only serialize unrelated transactions; they grant no authority.
func accountKeyLock(ctx context.Context, tx pgx.Tx, key string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key)
	return err
}

// All Google mutations lock flow, then subject key, email key, user, tokens/outbox.
// Password operations never acquire flow locks, avoiding inverse lock dependencies.
func (r *PostgresRepository) advanceGoogle(ctx context.Context, token, transport string, identity *GoogleIdentity, username, passwordHash string, s *Service, source string) (GoogleResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return GoogleResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	f, err := loadGoogleFlow(ctx, tx, token, transport, true)
	if err != nil {
		return GoogleResult{}, err
	}
	if identity != nil {
		if f.Status != "awaiting_proof" {
			return GoogleResult{}, ErrInvalidFlow
		}
		f.Subject, f.Email, f.Status = identity.Subject, identity.Email, "ready"
		if _, err = tx.Exec(ctx, `UPDATE auth_google_flows SET subject=$2,email=$3,email_key=$4,status='ready' WHERE id=$1`, f.ID, f.Subject, f.Email, NormalizeIdentifier(f.Email)); err != nil {
			return GoogleResult{}, err
		}
	} else if passwordHash != "" {
		if f.Status != "onboarding" {
			return GoogleResult{}, ErrInvalidFlow
		}
	} else if f.Status != "ready" {
		return GoogleResult{}, ErrInvalidFlow
	}

	if err = accountKeyLock(ctx, tx, "google:subject:"+f.Subject); err != nil {
		return GoogleResult{}, err
	}
	var linkedID string
	err = tx.QueryRow(ctx, `SELECT user_id FROM external_identities WHERE provider='google' AND subject=$1`, f.Subject).Scan(&linkedID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return GoogleResult{}, err
	}
	// A previously linked subject remains the identity even if its Google email changes.
	if linkedID == "" && identity != nil && !identity.Authoritative {
		if err = s.mailSource(ctx, source); err != nil {
			return GoogleResult{}, err
		}
		f.Status = "email_verification_required"
		if err = r.issueGoogleEmail(ctx, tx, f, s.recovery); err != nil {
			return GoogleResult{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE auth_google_flows SET status=$2 WHERE id=$1`, f.ID, f.Status); err != nil {
			return GoogleResult{}, err
		}
		if err = flowValidity(f.Status, f.ExpiresAt); err != nil {
			return GoogleResult{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return GoogleResult{}, err
		}
		return GoogleResult{Context: f.GoogleContext}, nil
	}
	result, err := r.resolveGoogleAccount(ctx, tx, &f, linkedID, username, passwordHash, s)
	if err != nil {
		return GoogleResult{}, err
	}
	// Recheck after any waits on account/email locks; expired contexts cannot mint sessions.
	if err = flowValidity(f.Status, f.ExpiresAt); err != nil {
		return GoogleResult{}, err
	}
	status := f.Status
	if result != nil {
		status = "consumed"
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_google_flows SET status=$2 WHERE id=$1`, f.ID, status); err != nil {
		return GoogleResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return GoogleResult{}, err
	}
	return GoogleResult{Context: f.GoogleContext, Account: result}, nil
}

func (r *PostgresRepository) resolveGoogleAccount(ctx context.Context, tx pgx.Tx, f *googleFlow, linkedID, username, passwordHash string, s *Service) (*Result, error) {
	var user User
	if linkedID == "" {
		if err := accountKeyLock(ctx, tx, "account:email:"+NormalizeIdentifier(f.Email)); err != nil {
			return nil, err
		}
	}
	query := `SELECT id,email,username,verified_at IS NOT NULL FROM users WHERE email_key=$1 FOR UPDATE`
	key := NormalizeIdentifier(f.Email)
	if linkedID != "" {
		query = `SELECT id,email,username,verified_at IS NOT NULL FROM users WHERE id=$1 FOR UPDATE`
		key = linkedID
	}
	err := tx.QueryRow(ctx, query, key).Scan(&user.ID, &user.Email, &user.Username, &user.Verified)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if user.ID != "" {
		var boundSubject string
		err = tx.QueryRow(ctx, `SELECT subject FROM external_identities WHERE provider='google' AND user_id=$1`, user.ID).Scan(&boundSubject)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if boundSubject != "" && boundSubject != f.Subject {
			return nil, ErrIdentityConflict
		}
	}
	if !user.Verified {
		if passwordHash == "" {
			f.Status = "onboarding"
			f.Username = user.Username
			return nil, nil
		}
		if user.ID == "" {
			err = tx.QueryRow(ctx, `INSERT INTO users (email,email_key,username,username_key,password_hash,verified_at)
                VALUES ($1,$2,$3,$4,$5,clock_timestamp()) RETURNING id,email,username`, f.Email, NormalizeIdentifier(f.Email), username, NormalizeIdentifier(username), passwordHash).Scan(&user.ID, &user.Email, &user.Username)
			if err != nil {
				return nil, uniqueError(err)
			}
		} else {
			if !strings.EqualFold(username, user.Username) {
				return nil, ErrInvalidInput
			}
			if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2,verified_at=clock_timestamp() WHERE id=$1`, user.ID, passwordHash); err != nil {
				return nil, err
			}
			if _, err = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=clock_timestamp() WHERE user_id=$1 AND revoked_at IS NULL`, user.ID); err != nil {
				return nil, err
			}
			for _, purpose := range []string{"verification", "reset"} {
				if err = r.invalidatePurpose(ctx, tx, user.ID, purpose); err != nil {
					return nil, err
				}
			}
		}
		user.Verified = true
	}
	if linkedID == "" {
		_, err = tx.Exec(ctx, `INSERT INTO external_identities (provider,subject,user_id) VALUES ('google',$1,$2)`, f.Subject, user.ID)
		if err != nil {
			return nil, googleIdentityError(err)
		}
	}
	token := newToken()
	result := &Result{Token: token, Session: Session{User: user}}
	err = tx.QueryRow(ctx, `INSERT INTO auth_sessions (user_id,token_hash,last_used_at,expires_at)
        VALUES ($1,$2,clock_timestamp(),clock_timestamp()+$3*interval '1 second') RETURNING id,expires_at`, user.ID, tokenHash(token), s.policy.SessionIdle.Seconds()).Scan(&result.Session.ID, &result.Session.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return result, nil
}
