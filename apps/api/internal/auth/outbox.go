package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"keepup/apps/api/internal/maildelivery"
)

// DeliverOne claims an account using SKIP LOCKED, then its next queued message.
// The user lock is held during bounded delivery so a replacement/consumption
// cannot commit between validity checking and sending. SMTP remains at least once.
func (s *Service) DeliverOne(ctx context.Context, sender maildelivery.Sender) (bool, error) {
	p := s.recovery.config.Policy
	ctx, cancel := context.WithTimeout(ctx, p.DeliveryTimeout+5*time.Second)
	defer cancel()
	tx, err := s.repo.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin mail delivery: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var userID string
	err = tx.QueryRow(ctx, `SELECT u.id FROM users u WHERE EXISTS (
        SELECT 1 FROM auth_tokens t JOIN auth_mail_outbox o ON o.token_id=t.id
        WHERE t.user_id=u.id AND o.status='pending' AND o.next_attempt_at<=clock_timestamp())
        ORDER BY u.id LIMIT 1 FOR UPDATE OF u SKIP LOCKED`).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim mail account: %w", err)
	}
	var id, tokenID, purpose, to string
	var payload []byte
	var failures int
	var invalid bool
	err = tx.QueryRow(ctx, `SELECT o.id,t.id,t.purpose,u.email,o.payload,o.failures,
        t.consumed_at IS NOT NULL OR t.invalidated_at IS NOT NULL OR t.expires_at<=clock_timestamp()
        FROM auth_mail_outbox o JOIN auth_tokens t ON t.id=o.token_id JOIN users u ON u.id=t.user_id
        WHERE t.user_id=$1 AND o.status='pending' AND o.next_attempt_at<=clock_timestamp()
        ORDER BY o.next_attempt_at,o.id LIMIT 1 FOR UPDATE OF o SKIP LOCKED`, userID).Scan(&id, &tokenID, &purpose, &to, &payload, &failures, &invalid)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim mail row: %w", err)
	}
	status := "cancelled"
	if !invalid {
		link, decryptErr := s.recovery.decrypt(tokenID, payload)
		status = "failed"
		if decryptErr == nil {
			subject, text := "Verify your KeepUp email", "Confirm your email to activate your account."
			if purpose == "reset" {
				subject, text = "Reset your KeepUp password", "Choose a new password. This logs out all devices."
			}
			deliveryCtx, stop := context.WithTimeout(ctx, p.DeliveryTimeout)
			sendErr := sender.Send(deliveryCtx, maildelivery.Message{ID: id, To: to, Subject: subject, Text: text + "\n\n" + link + "\n\nIf you did not request this, you can ignore this message.\n"})
			stop()
			if sendErr == nil {
				status = "sent"
			} else if failures < len(p.RetryDelays) {
				_, err = tx.Exec(ctx, `UPDATE auth_mail_outbox SET failures=failures+1,next_attempt_at=clock_timestamp()+$2*interval '1 second' WHERE id=$1`, id, p.RetryDelays[failures].Seconds())
				if err != nil {
					return true, fmt.Errorf("schedule mail retry: %w", err)
				}
				if err = tx.Commit(ctx); err != nil {
					return true, fmt.Errorf("commit mail retry: %w", err)
				}
				return true, nil
			}
		}
	}
	_, err = tx.Exec(ctx, `UPDATE auth_mail_outbox SET status=$2,payload=NULL,finished_at=clock_timestamp(),
        failures=failures+CASE WHEN $2='failed' THEN 1 ELSE 0 END WHERE id=$1`, id, status)
	if err != nil {
		return true, fmt.Errorf("finish mail delivery: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit mail delivery: %w", err)
	}
	return true, nil
}

// Cleanup removes expired credentials/counters after the configured retention.
// Retention covers all rate windows so cleanup cannot reset a live quota.
func (s *Service) Cleanup(ctx context.Context) error {
	retention := s.recovery.config.Policy.Retention.Seconds()
	for _, query := range []string{
		`DELETE FROM auth_tokens WHERE expires_at<clock_timestamp()-$1*interval '1 second'`,
		`DELETE FROM auth_sessions WHERE expires_at<clock_timestamp()-$1*interval '1 second' OR revoked_at<clock_timestamp()-$1*interval '1 second'`,
		`DELETE FROM auth_attempts WHERE window_started_at<clock_timestamp()-$1*interval '1 second'`,
	} {
		if _, err := s.repo.pool.Exec(ctx, query, retention); err != nil {
			return fmt.Errorf("clean authentication records: %w", err)
		}
	}
	// Wipe encrypted payloads as soon as an otherwise unprocessed link expires.
	_, err := s.repo.pool.Exec(ctx, `UPDATE auth_mail_outbox o SET status='cancelled',payload=NULL,finished_at=clock_timestamp()
        FROM auth_tokens t WHERE o.token_id=t.id AND o.status='pending'
        AND (t.expires_at<=clock_timestamp() OR t.consumed_at IS NOT NULL OR t.invalidated_at IS NOT NULL)`)
	if err != nil {
		return fmt.Errorf("clean expired mail payloads: %w", err)
	}
	return nil
}

// RunMail owns the delivery/cleanup loop; callers join it before closing the pool.
func (s *Service) RunMail(ctx context.Context, sender maildelivery.Sender, logger *slog.Logger) {
	poll := time.NewTicker(s.recovery.config.Policy.PollInterval)
	defer poll.Stop()
	cleanup := time.NewTicker(s.recovery.config.Policy.CleanupInterval)
	defer cleanup.Stop()
	for {
		// Limit each batch so continuous email traffic cannot starve cleanup/shutdown.
		for i := 0; i < 50 && ctx.Err() == nil; i++ {
			worked, err := s.DeliverOne(ctx, sender)
			if err != nil {
				if ctx.Err() == nil {
					logger.Error("account mail queue operation failed")
				}
				break
			}
			if !worked {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-cleanup.C:
			if err := s.Cleanup(ctx); err != nil && ctx.Err() == nil {
				logger.Error("account credential cleanup failed")
			}
		case <-poll.C:
		}
	}
}
