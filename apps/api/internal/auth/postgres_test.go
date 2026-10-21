package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func accountTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for account integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := fmt.Sprintf("auth_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// Only account migration, with no reads/writes to application schemas.
	for _, name := range []string{"0003_accounts_and_sessions.up.sql", "0004_auth_recovery_and_outbox.up.sql"} {
		sql, readErr := os.ReadFile(filepath.Join("../../../../db/migrations", name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func testRecoveryConfig() RecoveryConfig {
	p := DefaultRecoveryPolicy()
	p.MinRecoveryResponse = time.Nanosecond
	return RecoveryConfig{PublicURL: "https://keepup.example", EncryptionKey: bytes.Repeat([]byte{1}, 32), Policy: p}
}

func accountTestService(t *testing.T, pool *pgxpool.Pool, policy Policy) *Service {
	t.Helper()
	service, err := NewService(NewPostgresRepository(pool), policy, testRecoveryConfig())
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestPostgresAccounts(t *testing.T) {
	pool := accountTestPool(t)
	ctx := context.Background()
	policy := DefaultPolicy()
	policy.LoginAttempts = 1000
	policy.SignupAttempts = 1000
	service := accountTestService(t, pool, policy)
	input := SignupInput{"Case.Name+Tag@Example.com", "Keep-Up_1", "password1!"}
	created, err := service.Signup(ctx, input, "test")
	if err != nil {
		t.Fatal(err)
	}
	if created.Session.User.Verified || created.Session.User.Username != input.Username || len(created.Token) != 43 {
		t.Fatal("unexpected pending account")
	}
	var stored string
	if err = pool.QueryRow(ctx, "SELECT token_hash FROM auth_sessions WHERE id=$1", created.Session.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == created.Token || stored != tokenHash(created.Token) {
		t.Fatal("raw session credential persisted")
	}
	for _, identifier := range []string{"kEEP-uP_1", "case.name+tag@EXAMPLE.COM"} {
		if _, err = service.Login(ctx, identifier, input.Password, "test"); err != nil {
			t.Fatal(err)
		}
	}
	for _, identifier := range []string{input.Username, "unknown"} {
		if _, err = service.Login(ctx, identifier, "wrong", "test"); !errors.Is(err, ErrCredentials) {
			t.Fatal(err)
		}
	}
	if _, err = service.Signup(ctx, SignupInput{strings.ToLower(input.Email), "Another", "password1!"}, "test"); !errors.Is(err, ErrEmailTaken) {
		t.Fatal(err)
	}
	if _, err = service.Signup(ctx, SignupInput{"other@example.com", strings.ToLower(input.Username), "password1!"}, "test"); !errors.Is(err, ErrUsernameTaken) {
		t.Fatal(err)
	}
	for _, column := range []string{"email", "username"} {
		if _, err = pool.Exec(ctx, "UPDATE users SET "+column+"=$1 WHERE id=$2", "changed", created.Session.User.ID); err == nil {
			t.Fatal("identifier update allowed")
		}
	}
	if err = RequireActive(created.Session); !errors.Is(err, ErrPending) {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE users SET verified_at=now() WHERE id=$1", created.Session.User.ID); err != nil {
		t.Fatal(err)
	}
	active, err := service.Authenticate(ctx, created.Token)
	if err != nil || !active.User.Verified {
		t.Fatal("activation not visible", err)
	}
	rotated, err := service.Rotate(ctx, created.Token)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Session.ID != created.Session.ID || rotated.Token == created.Token {
		t.Fatal("rotation changed session identity or retained secret")
	}
	if _, err = service.Authenticate(ctx, created.Token); !errors.Is(err, ErrSession) {
		t.Fatal("old credential survived rotation", err)
	}
	if err = service.Logout(ctx, rotated.Token, false); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Authenticate(ctx, rotated.Token); !errors.Is(err, ErrSession) {
		t.Fatal("revoked session accepted", err)
	}
	login, err := service.Login(ctx, input.Email, input.Password, "test")
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.Login(ctx, input.Username, input.Password, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Logout(ctx, login.Token, true); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Authenticate(ctx, other.Token); !errors.Is(err, ErrSession) {
		t.Fatal("logout everywhere missed session", err)
	}
	fresh, err := service.Login(ctx, input.Username, input.Password, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE auth_sessions SET expires_at=now()-interval '1 second' WHERE id=$1", fresh.Session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Authenticate(ctx, fresh.Token); !errors.Is(err, ErrSession) {
		t.Fatal("expired session revived", err)
	}
	current, err := service.Login(ctx, input.Username, input.Password, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE auth_sessions SET expires_at=now()+interval '5 seconds' WHERE id=$1", current.Session.ID); err != nil {
		t.Fatal(err)
	}
	extended, err := service.Authenticate(ctx, current.Token)
	if err != nil || extended.ExpiresAt.Before(time.Now().Add(29*24*time.Hour)) {
		t.Fatal("idle expiry not extended", err)
	}
	start := make(chan struct{})
	results := make(chan Result, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			r, err := service.Rotate(ctx, current.Token)
			results <- r
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	winners := 0
	for err := range errs {
		if err == nil {
			winners++
		} else if !errors.Is(err, ErrSession) {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatal("rotation replay created multiple credentials", winners)
	}
	for result := range results {
		if result.Token != "" {
			if _, err = service.Authenticate(ctx, result.Token); err != nil {
				t.Fatal("winning credential rejected", err)
			}
		}
	}
}

func TestPostgresConcurrentSignup(t *testing.T) {
	pool := accountTestPool(t)
	service := accountTestService(t, pool, DefaultPolicy())
	ctx := context.Background()
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, input := range []SignupInput{{"same@example.com", "First", "password1!"}, {"SAME@example.com", "Second", "password1!"}} {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, err := service.Signup(ctx, input, "test"); results <- err }()
	}
	close(start)
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrEmailTaken) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("concurrent uniqueness failed", success, conflict)
	}
	var users, sessions int
	if err := pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM auth_sessions)").Scan(&users, &sessions); err != nil {
		t.Fatal(err)
	}
	if users != 1 || sessions != 1 {
		t.Fatal("signup transaction leaked records", users, sessions)
	}
}

func TestPostgresAttemptLimits(t *testing.T) {
	pool := accountTestPool(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := repo.allowAttempt(ctx, "test", time.Minute, 2); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.allowAttempt(ctx, "test", time.Minute, 2); !errors.Is(err, ErrRateLimited) {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE auth_attempts SET window_started_at=now()-interval '2 minutes'"); err != nil {
		t.Fatal(err)
	}
	if err := repo.allowAttempt(ctx, "test", time.Minute, 2); err != nil {
		t.Fatal("window did not reset", err)
	}
}
