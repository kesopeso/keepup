package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"keepup/apps/api/internal/maildelivery"
)

func TestOutboxRetryAndSupersession(t *testing.T) {
	s, pool := recoveryTestService(t)
	ctx := context.Background()
	created := signupForRecovery(t, s, "MailRetry")
	fake := &maildelivery.Fake{}
	fake.SetError(errors.New("simulated provider failure"))
	for failure := 0; failure <= len(s.recovery.config.Policy.RetryDelays); failure++ {
		worked, err := s.DeliverOne(ctx, fake)
		if err != nil || !worked {
			t.Fatal(worked, err)
		}
		var status string
		var count int
		var payload bool
		var next time.Time
		if err = pool.QueryRow(ctx, `SELECT status,failures,payload IS NOT NULL,next_attempt_at FROM auth_mail_outbox`).Scan(&status, &count, &payload, &next); err != nil {
			t.Fatal(err)
		}
		if count != failure+1 {
			t.Fatal("incorrect retry count", count)
		}
		if failure < len(s.recovery.config.Policy.RetryDelays) {
			if status != "pending" || !payload || next.Before(time.Now().Add(s.recovery.config.Policy.RetryDelays[failure]-time.Second)) {
				t.Fatal("retry policy not honored", status, next)
			}
			if worked, err = s.DeliverOne(ctx, fake); err != nil || worked {
				t.Fatal("retried before due time", worked, err)
			}
			if _, err = pool.Exec(ctx, `UPDATE auth_mail_outbox SET next_attempt_at=now()-interval '1 second'`); err != nil {
				t.Fatal(err)
			}
		} else if status != "failed" || payload {
			t.Fatal("terminal delivery retained secret or kept retrying", status)
		}
	}
	pending, err := s.Authenticate(ctx, created.Token)
	if err != nil || pending.User.Verified {
		t.Fatal("failed delivery activated account", err)
	}
	fake.SetError(nil)
	ageMail(t, pool, created.Session.User.ID)
	if err = s.ResendVerification(ctx, created.Token, "test"); err != nil {
		t.Fatal(err)
	}
	old := queuedRecoveryToken(t, s, created.Session.User.ID, "verification")
	// A provider failure followed by replacement must not retry the old link.
	fake.SetError(io.ErrUnexpectedEOF)
	if _, err = s.DeliverOne(ctx, fake); err != nil {
		t.Fatal(err)
	}
	ageMail(t, pool, created.Session.User.ID)
	if err = s.ResendVerification(ctx, created.Token, "test"); err != nil {
		t.Fatal(err)
	}
	fake.SetError(nil)
	if _, err = s.DeliverOne(ctx, fake); err != nil {
		t.Fatal(err)
	}
	if len(fake.Messages()) != 1 {
		t.Fatal("delivery count", len(fake.Messages()))
	}
	if err = s.ConfirmVerification(ctx, old, "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("superseded delivery link accepted", err)
	}
}

type blockingDelivery struct {
	entered chan struct{}
	release chan struct{}
}

func (s *blockingDelivery) Send(ctx context.Context, _ maildelivery.Message) error {
	close(s.entered)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return nil
	}
}

func TestOutboxConcurrentWorkersAndShutdown(t *testing.T) {
	s, _ := recoveryTestService(t)
	signupForRecovery(t, s, "Worker")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sender := &blockingDelivery{entered: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := s.DeliverOne(ctx, sender); done <- err }()
	select {
	case <-sender.entered:
	case <-ctx.Done():
		t.Fatal("delivery never started")
	}
	if worked, err := s.DeliverOne(ctx, &maildelivery.Fake{}); err != nil || worked {
		t.Fatal("second worker claimed in-flight message", worked, err)
	}
	close(sender.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{})
	workerCtx, stop := context.WithCancel(context.Background())
	go func() {
		defer close(stopped)
		s.RunMail(workerCtx, &maildelivery.Fake{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	stop()
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("mail loop did not stop")
	}
}

func TestRecoveryCleanup(t *testing.T) {
	s, pool := recoveryTestService(t)
	ctx := context.Background()
	created := signupForRecovery(t, s, "Cleanup")
	if _, err := pool.Exec(ctx, `UPDATE auth_tokens SET expires_at=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	fake := &maildelivery.Fake{}
	if _, err := s.DeliverOne(ctx, fake); err != nil {
		t.Fatal(err)
	}
	if len(fake.Messages()) != 0 {
		t.Fatal("expired link delivered")
	}
	if err := s.repo.allowAttempt(ctx, "retained-quota", time.Hour, 5); err != nil {
		t.Fatal(err)
	}
	if err := s.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	var payloads, count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_mail_outbox WHERE payload IS NOT NULL`).Scan(&payloads); err != nil || payloads != 0 {
		t.Fatal("expired payload retained", err)
	}
	if err := pool.QueryRow(ctx, `SELECT attempts FROM auth_attempts WHERE key_hash=$1`, tokenHash("retained-quota")).Scan(&count); err != nil || count != 1 {
		t.Fatal("cleanup reset live rate limit", err)
	}
	for _, query := range []string{
		`UPDATE auth_tokens SET expires_at=now()-interval '8 days'`,
		`UPDATE auth_sessions SET expires_at=now()-interval '8 days'`,
		`UPDATE auth_attempts SET window_started_at=now()-interval '8 days'`,
	} {
		if _, err := pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM auth_tokens)+(SELECT count(*) FROM auth_mail_outbox)+(SELECT count(*) FROM auth_sessions)+(SELECT count(*) FROM auth_attempts)`).Scan(&count); err != nil || count != 0 {
		t.Fatal("expired records retained", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE id=$1`, created.Session.User.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("cleanup deleted account", err)
	}
}
