package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"keepup/apps/api/internal/maildelivery"
)

func recoveryTestService(t *testing.T) (*Service, *pgxpool.Pool) {
	t.Helper()
	pool := accountTestPool(t)
	cfg := testRecoveryConfig()
	cfg.Policy.MailPerAccount = 100
	cfg.Policy.MailPerSource = 100
	cfg.Policy.ConfirmAttempts = 100
	policy := DefaultPolicy()
	policy.LoginAttempts = 100
	policy.SignupAttempts = 100
	s, err := NewService(NewPostgresRepository(pool), policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s, pool
}

func signupForRecovery(t *testing.T, s *Service, name string) Result {
	t.Helper()
	r, err := s.Signup(context.Background(), SignupInput{name + "@example.com", name, "password1!"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func queuedRecoveryToken(t *testing.T, s *Service, userID, purpose string) string {
	t.Helper()
	var id string
	var payload []byte
	err := s.repo.pool.QueryRow(context.Background(), `SELECT t.id,o.payload FROM auth_tokens t JOIN auth_mail_outbox o ON o.token_id=t.id
        WHERE t.user_id=$1 AND t.purpose=$2 AND t.consumed_at IS NULL AND t.invalidated_at IS NULL`, userID, purpose).Scan(&id, &payload)
	if err != nil {
		t.Fatal(err)
	}
	link, err := s.recovery.decrypt(id, payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), link) {
		t.Fatal("mail link persisted without encryption")
	}
	return tokenFromLink(t, link)
}

func tokenFromLink(t *testing.T, link string) string {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if u.RawQuery != "" {
		t.Fatal("token leaked into query")
	}
	values, err := url.ParseQuery(u.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	token := values.Get("token")
	if len(token) != 43 {
		t.Fatal("invalid token shape")
	}
	return token
}

func ageMail(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE auth_tokens SET created_at=now()-interval '2 minutes' WHERE user_id=$1`, userID); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationActivation(t *testing.T) {
	s, pool := recoveryTestService(t)
	ctx := context.Background()
	created := signupForRecovery(t, s, "Verify")
	token := queuedRecoveryToken(t, s, created.Session.User.ID, "verification")
	fake := &maildelivery.Fake{}
	worked, err := s.DeliverOne(ctx, fake)
	if err != nil || !worked {
		t.Fatal(worked, err)
	}
	messages := fake.Messages()
	if len(messages) != 1 || messages[0].To != "Verify@example.com" || !strings.Contains(messages[0].Text, "/auth/verify#token="+token) {
		t.Fatal("incorrect verification mail")
	}
	if err = s.ConfirmVerification(ctx, token, "test"); err != nil {
		t.Fatal(err)
	}
	active, err := s.Authenticate(ctx, created.Token)
	if err != nil || RequireActive(active) != nil {
		t.Fatal("pending session did not observe activation", err)
	}
	if err = s.ConfirmVerification(ctx, token, "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("verification replay accepted", err)
	}
	if err = s.ResetPassword(ctx, token, "different1!", "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("verification used as reset", err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM auth_sessions").Scan(&count); err != nil || count != 1 {
		t.Fatal("verification created a new session", count, err)
	}
	pending := signupForRecovery(t, s, "Expired")
	expired := queuedRecoveryToken(t, s, pending.Session.User.ID, "verification")
	if _, err = pool.Exec(ctx, `UPDATE auth_tokens SET expires_at=now()-interval '1 second' WHERE user_id=$1`, pending.Session.User.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.ConfirmVerification(ctx, expired, "test"); !errors.Is(err, ErrTokenExpired) {
		t.Fatal("expired verification accepted", err)
	}
	unverified, err := s.Authenticate(ctx, pending.Token)
	if err != nil || !errors.Is(RequireActive(unverified), ErrPending) {
		t.Fatal("expired link activated account", err)
	}
}

func TestTokenReplacementAndCooldown(t *testing.T) {
	s, pool := recoveryTestService(t)
	ctx := context.Background()
	created := signupForRecovery(t, s, "Replacement")
	old := queuedRecoveryToken(t, s, created.Session.User.ID, "verification")
	if err := s.ResendVerification(ctx, created.Token, "test"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("resend cooldown bypassed", err)
	}
	ageMail(t, pool, created.Session.User.ID)
	if err := s.ResendVerification(ctx, created.Token, "test"); err != nil {
		t.Fatal(err)
	}
	current := queuedRecoveryToken(t, s, created.Session.User.ID, "verification")
	if old == current {
		t.Fatal("replacement reused token")
	}
	if err := s.ConfirmVerification(ctx, old, "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("superseded link accepted", err)
	}
	fake := &maildelivery.Fake{}
	if _, err := s.DeliverOne(ctx, fake); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeliverOne(ctx, fake); err != nil {
		t.Fatal(err)
	}
	if len(fake.Messages()) != 1 || !strings.Contains(fake.Messages()[0].Text, current) {
		t.Fatal("superseded queued mail delivered")
	}
	var payloads int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM auth_mail_outbox WHERE payload IS NOT NULL").Scan(&payloads); err != nil || payloads != 0 {
		t.Fatal("terminal mail retained a secret", err)
	}
	if err := s.ForgotPassword(ctx, "unknown", "test"); err != nil {
		t.Fatal("unknown recovery exposed account absence", err)
	}
	if err := s.ForgotPassword(ctx, "Replacement", "test"); err != nil {
		t.Fatal(err)
	}
	reset := queuedRecoveryToken(t, s, created.Session.User.ID, "reset")
	if err := s.ForgotPassword(ctx, "Replacement", "test"); err != nil {
		t.Fatal("cooldown disclosed account existence", err)
	}
	ageMail(t, pool, created.Session.User.ID)
	if err := s.ForgotPassword(ctx, "Replacement", "test"); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetPassword(ctx, reset, "different1!", "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("old reset survived replacement", err)
	}
}

func TestResetAndPasswordChangeRevocation(t *testing.T) {
	s, pool := recoveryTestService(t)
	ctx := context.Background()
	created := signupForRecovery(t, s, "Passwords")
	other, err := s.Login(ctx, "Passwords", "password1!", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ForgotPassword(ctx, "Passwords", "test"); err != nil {
		t.Fatal(err)
	}
	reset := queuedRecoveryToken(t, s, created.Session.User.ID, "reset")
	if err = s.ResetPassword(ctx, reset, "weak", "test"); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(ctx, created.Token); err != nil {
		t.Fatal("invalid password consumed token or revoked session", err)
	}
	if err = s.ResetPassword(ctx, reset, "new-password1!", "test"); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{created.Token, other.Token} {
		if _, err = s.Authenticate(ctx, token); !errors.Is(err, ErrSession) {
			t.Fatal("reset failed to revoke session", err)
		}
	}
	if _, err = s.Login(ctx, "Passwords", "password1!", "test"); !errors.Is(err, ErrCredentials) {
		t.Fatal("old password survived reset", err)
	}
	login, err := s.Login(ctx, "Passwords", "new-password1!", "test")
	if err != nil {
		t.Fatal(err)
	}
	if login.Session.User.Verified {
		t.Fatal("reset incorrectly verified account")
	}
	if _, err = s.ChangePassword(ctx, login.Token, "new-password1!", "changed-password1!", "test"); !errors.Is(err, ErrPending) {
		t.Fatal("pending account changed password", err)
	}
	verification := queuedRecoveryToken(t, s, created.Session.User.ID, "verification")
	if err = s.ConfirmVerification(ctx, verification, "test"); err != nil {
		t.Fatal(err)
	}
	second, err := s.Login(ctx, "Passwords", "new-password1!", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangePassword(ctx, login.Token, "wrong", "changed-password1!", "test"); !errors.Is(err, ErrCredentials) {
		t.Fatal("wrong current password accepted", err)
	}
	if _, err = s.Authenticate(ctx, login.Token); err != nil {
		t.Fatal("wrong current password revoked valid session", err)
	}
	ageMail(t, pool, created.Session.User.ID)
	if err = s.ForgotPassword(ctx, "Passwords", "test"); err != nil {
		t.Fatal(err)
	}
	outstanding := queuedRecoveryToken(t, s, created.Session.User.ID, "reset")
	changed, err := s.ChangePassword(ctx, login.Token, "new-password1!", "changed-password1!", "test")
	if err != nil {
		t.Fatal(err)
	}
	if changed.Session.ID != login.Session.ID || changed.Token == login.Token {
		t.Fatal("password change did not retain/rotate current session")
	}
	for _, token := range []string{login.Token, second.Token} {
		if _, err = s.Authenticate(ctx, token); !errors.Is(err, ErrSession) {
			t.Fatal("old session credential survived password change", err)
		}
	}
	if _, err = s.Authenticate(ctx, changed.Token); err != nil {
		t.Fatal("rotated current session rejected", err)
	}
	if err = s.ResetPassword(ctx, outstanding, "attacker1!", "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("outstanding reset survived password change", err)
	}
	if _, err = s.Login(ctx, "Passwords", "changed-password1!", "test"); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentResetConsumption(t *testing.T) {
	s, _ := recoveryTestService(t)
	ctx := context.Background()
	created := signupForRecovery(t, s, "Concurrent")
	if err := s.ForgotPassword(ctx, "Concurrent", "test"); err != nil {
		t.Fatal(err)
	}
	token := queuedRecoveryToken(t, s, created.Session.User.ID, "reset")
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- s.ResetPassword(ctx, token, "new-password1!", "test") }()
	}
	close(start)
	wg.Wait()
	close(results)
	winners, replays := 0, 0
	for err := range results {
		if err == nil {
			winners++
		} else if errors.Is(err, ErrInvalidToken) {
			replays++
		} else {
			t.Fatal(err)
		}
	}
	if winners != 1 || replays != 1 {
		t.Fatal("token consumption was not atomic", winners, replays)
	}
}

func TestRecoveryEncryptionBinding(t *testing.T) {
	cfg := testRecoveryConfig()
	runtime, err := newRecovery(cfg)
	if err != nil {
		t.Fatal(err)
	}
	encrypted := runtime.encrypt("first", "secret-link")
	if _, err = runtime.decrypt("second", encrypted); err == nil {
		t.Fatal("encrypted mail could be transplanted")
	}
	encrypted[len(encrypted)-1] ^= 1
	if _, err = runtime.decrypt("first", encrypted); err == nil {
		t.Fatal("tampered mail accepted")
	}
	cfg.EncryptionKey = nil
	if _, err = newRecovery(cfg); err == nil {
		t.Fatal("missing encryption key accepted")
	}
	cfg = testRecoveryConfig()
	cfg.Policy.Retention = cfg.Policy.MailWindow - time.Nanosecond
	if err = cfg.Validate(); err == nil {
		t.Fatal("cleanup could reset a live quota")
	}
}

func TestRecoveryQuotasAndExpiredReset(t *testing.T) {
	pool := accountTestPool(t)
	cfg := testRecoveryConfig()
	cfg.Policy.MailPerAccount = 2
	cfg.Policy.MailPerSource = 100
	cfg.Policy.ConfirmAttempts = 2
	s, err := NewService(NewPostgresRepository(pool), DefaultPolicy(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created := signupForRecovery(t, s, "Quota")
	ageMail(t, pool, created.Session.User.ID)
	if err = s.ResendVerification(ctx, created.Token, "test"); err != nil {
		t.Fatal(err)
	}
	ageMail(t, pool, created.Session.User.ID)
	if err = s.ResendVerification(ctx, created.Token, "test"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("account mail quota bypassed", err)
	}
	if err = s.ForgotPassword(ctx, "Quota", "test"); err != nil {
		t.Fatal("mail quota disclosed account existence", err)
	}
	if err = s.ForgotPassword(ctx, "missing", "test"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM auth_tokens").Scan(&count); err != nil || count != 2 {
		t.Fatal("suppressed recovery queued email", count, err)
	}
	for i := 0; i < 2; i++ {
		if err = s.ConfirmVerification(ctx, strings.Repeat("x", 43), "limited"); !errors.Is(err, ErrInvalidToken) {
			t.Fatal(err)
		}
	}
	if err = s.ConfirmVerification(ctx, strings.Repeat("x", 43), "limited"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("confirmation attempts unlimited", err)
	}
	// Independent source quota applies equally to known and absent identifiers.
	otherCfg := testRecoveryConfig()
	otherCfg.Policy.MailPerSource = 1
	other, err := NewService(NewPostgresRepository(pool), DefaultPolicy(), otherCfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = other.ForgotPassword(ctx, "missing", "new-source"); err != nil {
		t.Fatal(err)
	}
	if err = other.ForgotPassword(ctx, "Quota", "new-source"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("source mail quota bypassed", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE auth_tokens SET created_at=now()-interval '2 hours'`); err != nil {
		t.Fatal(err)
	}
	if err = s.ForgotPassword(ctx, "Quota", "test"); err != nil {
		t.Fatal(err)
	}
	token := queuedRecoveryToken(t, s, created.Session.User.ID, "reset")
	if _, err = pool.Exec(ctx, `UPDATE auth_tokens SET expires_at=now()-interval '1 second' WHERE purpose='reset'`); err != nil {
		t.Fatal(err)
	}
	if err = s.ResetPassword(ctx, token, "new-password1!", "expiry-source"); !errors.Is(err, ErrTokenExpired) {
		t.Fatal("expired reset accepted", err)
	}
	if _, err = s.Authenticate(ctx, created.Token); err != nil {
		t.Fatal("expired reset revoked valid session", err)
	}
}

func TestSignupRollsBackWhenQueueInsertFails(t *testing.T) {
	s, pool := recoveryTestService(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `ALTER TABLE auth_mail_outbox ADD CONSTRAINT simulated_queue_failure CHECK (false)`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Signup(ctx, SignupInput{"atomic@example.com", "Atomic", "password1!"}, "test"); err == nil {
		t.Fatal("signup ignored queue failure")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users)+(SELECT count(*) FROM auth_sessions)+(SELECT count(*) FROM auth_tokens)`).Scan(&count); err != nil || count != 0 {
		t.Fatal("signup persisted without queued verification", count, err)
	}
}
