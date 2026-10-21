package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"keepup/apps/api/internal/maildelivery"
)

type fakeGoogleProof struct{}

func (fakeGoogleProof) Verify(_ context.Context, proof string, _ []string) (GoogleIdentity, error) {
	var identity GoogleIdentity
	if err := json.Unmarshal([]byte(proof), &identity); err != nil {
		return identity, ErrGoogleCredential
	}
	return identity, nil
}

func googleTestService(t *testing.T) *Service {
	t.Helper()
	s, _ := recoveryTestService(t)
	cfg := DefaultGoogleConfig()
	cfg.WebClientIDs = []string{"web.apps.googleusercontent.com"}
	cfg.NativeClientIDs = []string{"native.apps.googleusercontent.com"}
	if err := WithGoogle(cfg, fakeGoogleProof{})(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func googleProof(t *testing.T, start GoogleStart, subject, email string, authoritative bool) string {
	t.Helper()
	raw, err := json.Marshal(GoogleIdentity{Subject: subject, Email: email, Nonce: start.Nonce, Authoritative: authoritative})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func beginGoogle(t *testing.T, s *Service, subject, email string, authoritative bool) (GoogleStart, GoogleResult) {
	t.Helper()
	flow, err := s.StartGoogle(context.Background(), "web", "test")
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ProveGoogle(context.Background(), flow.Token, "web", googleProof(t, flow, subject, email, authoritative), "test")
	if err != nil {
		t.Fatal(err)
	}
	return flow, result
}

func googleEmailToken(t *testing.T, s *Service, flow GoogleStart) string {
	t.Helper()
	var id string
	var payload []byte
	err := s.repo.pool.QueryRow(context.Background(), `SELECT t.id,o.payload FROM auth_tokens t JOIN auth_google_flows f ON f.id=t.google_flow_id
        JOIN auth_mail_outbox o ON o.token_id=t.id WHERE f.token_hash=$1 AND t.invalidated_at IS NULL AND t.consumed_at IS NULL`, tokenHash(flow.Token)).Scan(&id, &payload)
	if err != nil {
		t.Fatal(err)
	}
	link, err := s.recovery.decrypt(id, payload)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), link) {
		t.Fatal("unencrypted challenge")
	}
	return tokenFromLink(t, link)
}

func TestGoogleOnboardingAndSubjectIdentity(t *testing.T) {
	s := googleTestService(t)
	ctx := context.Background()
	flow, result := beginGoogle(t, s, "new-subject", "Case.Name@gmail.com", true)
	if result.Account != nil || result.Context.Status != "onboarding" {
		t.Fatalf("%+v", result)
	}
	if _, err := s.Authenticate(ctx, flow.Token); !errors.Is(err, ErrSession) {
		t.Fatal("flow granted session", err)
	}
	if _, err := s.GoogleContext(ctx, flow.Token, "native"); !errors.Is(err, ErrInvalidFlow) {
		t.Fatal("transport binding failed", err)
	}
	if _, err := s.CompleteGoogle(ctx, flow.Token, "web", "NewUser", "weak", "test"); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
	completed, err := s.CompleteGoogle(ctx, flow.Token, "web", "NewUser", "password1!", "test")
	if err != nil || completed.Account == nil || !completed.Account.Session.User.Verified {
		t.Fatalf("%+v %v", completed, err)
	}
	if _, err = s.Login(ctx, "newuser", "password1!", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteGoogle(ctx, flow.Token, "web", "Again", "password1!", "test"); !errors.Is(err, ErrInvalidFlow) {
		t.Fatal("completion replay", err)
	}
	// Google profile email changes never retarget a linked provider subject.
	_, login := beginGoogle(t, s, "new-subject", "Different@third.example", false)
	if login.Account == nil || login.Account.Session.User.ID != completed.Account.Session.User.ID || login.Account.Session.User.Email != "Case.Name@gmail.com" {
		t.Fatalf("subject identity changed: %+v", login)
	}
	var count int
	if err = s.repo.pool.QueryRow(ctx, `SELECT count(*) FROM auth_tokens`).Scan(&count); err != nil || count != 0 {
		t.Fatal("authoritative signup queued email", count, err)
	}
}

func TestGoogleActiveLinkAndIdentityConflict(t *testing.T) {
	s := googleTestService(t)
	ctx := context.Background()
	account := signupForRecovery(t, s, "Existing")
	if err := s.ConfirmVerification(ctx, queuedRecoveryToken(t, s, account.Session.User.ID, "verification"), "test"); err != nil {
		t.Fatal(err)
	}
	_, linked := beginGoogle(t, s, "subject-existing", "EXISTING@example.com", true)
	if linked.Account == nil || linked.Account.Session.User.ID != account.Session.User.ID {
		t.Fatalf("%+v", linked)
	}
	if _, err := s.Login(ctx, "Existing", "password1!", "test"); err != nil {
		t.Fatal("password replaced", err)
	}
	flow, err := s.StartGoogle(ctx, "web", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ProveGoogle(ctx, flow.Token, "web", googleProof(t, flow, "different-subject", "existing@example.com", true), "test"); !errors.Is(err, ErrIdentityConflict) {
		t.Fatal("provider collision allowed", err)
	}
	if _, err = s.Authenticate(ctx, account.Token); err != nil {
		t.Fatal("active prior session was revoked", err)
	}
}

func TestGooglePendingClaim(t *testing.T) {
	s := googleTestService(t)
	ctx := context.Background()
	prior := signupForRecovery(t, s, "Pending")
	oldVerification := queuedRecoveryToken(t, s, prior.Session.User.ID, "verification")
	if err := s.ForgotPassword(ctx, "Pending", "test"); err != nil {
		t.Fatal(err)
	}
	oldReset := queuedRecoveryToken(t, s, prior.Session.User.ID, "reset")
	flow, onboarding := beginGoogle(t, s, "claim-subject", "pending@example.com", true)
	if onboarding.Context.Username != "Pending" || onboarding.Account != nil {
		t.Fatalf("%+v", onboarding)
	}
	if _, err := s.CompleteGoogle(ctx, flow.Token, "web", "Changed", "newpassword2!", "test"); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("immutable username changed", err)
	}
	completed, err := s.CompleteGoogle(ctx, flow.Token, "web", "pending", "newpassword2!", "test")
	if err != nil || completed.Account == nil || completed.Account.Session.User.ID != prior.Session.User.ID || completed.Account.Session.User.Username != "Pending" {
		t.Fatalf("%+v %v", completed, err)
	}
	if _, err = s.Authenticate(ctx, prior.Token); !errors.Is(err, ErrSession) {
		t.Fatal("pre-claim session survived", err)
	}
	if _, err = s.Login(ctx, "Pending", "password1!", "test"); !errors.Is(err, ErrCredentials) {
		t.Fatal("pre-claim password survived", err)
	}
	if err = s.ConfirmVerification(ctx, oldVerification, "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("pre-claim verification survived", err)
	}
	if err = s.ResetPassword(ctx, oldReset, "attacker1!", "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("pre-claim reset survived", err)
	}
	if _, err = s.Login(ctx, "Pending", "newpassword2!", "test"); err != nil {
		t.Fatal(err)
	}
}

func TestGoogleEmailChallengeAndDelivery(t *testing.T) {
	s := googleTestService(t)
	ctx := context.Background()
	flow, pending := beginGoogle(t, s, "external-subject", "Outside@third.example", false)
	if pending.Context.Status != "email_verification_required" || pending.Account != nil {
		t.Fatalf("%+v", pending)
	}
	firstToken := googleEmailToken(t, s, flow)
	if _, err := s.ExchangeGoogle(ctx, flow.Token, "web", "test"); !errors.Is(err, ErrInvalidFlow) {
		t.Fatal("challenge bypass", err)
	}
	if err := s.ConfirmVerification(ctx, firstToken, "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("purpose confusion", err)
	}
	if err := s.ResendGoogleEmail(ctx, flow.Token, "web", "test"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("missing cooldown", err)
	}
	if _, err := s.repo.pool.Exec(ctx, `UPDATE auth_tokens SET created_at=now()-interval '2 minutes' WHERE purpose='google_email'`); err != nil {
		t.Fatal(err)
	}
	if err := s.ResendGoogleEmail(ctx, flow.Token, "web", "test"); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmGoogleEmail(ctx, firstToken, "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("superseded challenge accepted", err)
	}
	secondToken := googleEmailToken(t, s, flow)
	sender := &maildelivery.Fake{}
	if worked, err := s.DeliverOne(ctx, sender); err != nil || !worked {
		t.Fatal("Google outbox delivery", worked, err)
	}
	messages := sender.Messages()
	if len(messages) != 1 || messages[0].To != "Outside@third.example" || !strings.Contains(messages[0].Text, secondToken) {
		t.Fatalf("wrong delivery: %+v", messages)
	}
	if err := s.ConfirmGoogleEmail(ctx, secondToken, "test"); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmGoogleEmail(ctx, secondToken, "test"); !errors.Is(err, ErrInvalidToken) {
		t.Fatal("challenge replay", err)
	}
	context, err := s.GoogleContext(ctx, flow.Token, "web")
	if err != nil || context.Status != "ready" {
		t.Fatal(context, err)
	}
	var sessions, users int
	if err = s.repo.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM auth_sessions),(SELECT count(*) FROM users)`).Scan(&sessions, &users); err != nil || sessions != 0 || users != 0 {
		t.Fatal("confirmation signed in opener", sessions, users, err)
	}
	onboarding, err := s.ExchangeGoogle(ctx, flow.Token, "web", "test")
	if err != nil || onboarding.Context.Status != "onboarding" {
		t.Fatal(onboarding, err)
	}
	if _, err = s.CompleteGoogle(ctx, flow.Token, "web", "Outside", "password1!", "test"); err != nil {
		t.Fatal(err)
	}
}

func TestGoogleFlowExpiryNonceAndReplay(t *testing.T) {
	s := googleTestService(t)
	ctx := context.Background()
	start, err := s.StartGoogle(ctx, "native", "test")
	if err != nil {
		t.Fatal(err)
	}
	wrong := googleProof(t, GoogleStart{Nonce: newToken()}, "subject", "user@gmail.com", true)
	if _, err = s.ProveGoogle(ctx, start.Token, "native", wrong, "test"); !errors.Is(err, ErrGoogleCredential) {
		t.Fatal("nonce not checked", err)
	}
	if _, err = s.ProveGoogle(ctx, start.Token, "native", googleProof(t, start, "subject", "user@gmail.com", true), "test"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ProveGoogle(ctx, start.Token, "native", googleProof(t, start, "subject", "user@gmail.com", true), "test"); !errors.Is(err, ErrInvalidFlow) {
		t.Fatal("proof replay", err)
	}
	if _, err = s.repo.pool.Exec(ctx, `UPDATE auth_google_flows SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, tokenHash(start.Token)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteGoogle(ctx, start.Token, "native", "Expired", "password1!", "test"); !errors.Is(err, ErrFlowExpired) {
		t.Fatal("expired completion", err)
	}
	if _, err = s.GoogleContext(ctx, start.Token, "native"); !errors.Is(err, ErrFlowExpired) {
		t.Fatal(err)
	}
	if _, err = s.GoogleContext(ctx, newToken(), "native"); !errors.Is(err, ErrInvalidFlow) {
		t.Fatal(err)
	}
}

func TestGoogleConcurrentCompletion(t *testing.T) {
	for _, sameFlow := range []bool{true, false} {
		t.Run(map[bool]string{true: "single context", false: "two contexts"}[sameFlow], func(t *testing.T) {
			s := googleTestService(t)
			ctx := context.Background()
			first, _ := beginGoogle(t, s, "race-subject", "race@gmail.com", true)
			second := first
			if !sameFlow {
				second, _ = beginGoogle(t, s, "race-subject", "race@gmail.com", true)
			}
			gate := make(chan struct{})
			results := make(chan error, 2)
			var wg sync.WaitGroup
			for i, flow := range []GoogleStart{first, second} {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-gate
					_, err := s.CompleteGoogle(ctx, flow.Token, "web", []string{"First", "Second"}[i], "password1!", "test")
					results <- err
				}()
			}
			close(gate)
			wg.Wait()
			close(results)
			successes := 0
			for err := range results {
				if err == nil {
					successes++
				} else if !sameFlow || !errors.Is(err, ErrInvalidFlow) {
					t.Fatal(err)
				}
			}
			expected := 2
			if sameFlow {
				expected = 1
			}
			if successes != expected {
				t.Fatal("wrong race result", successes)
			}
			var users, identities int
			if err := s.repo.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM external_identities)`).Scan(&users, &identities); err != nil || users != 1 || identities != 1 {
				t.Fatal("duplicate identity", users, identities, err)
			}
		})
	}
}

func TestGooglePasswordSignupRace(t *testing.T) {
	s := googleTestService(t)
	ctx := context.Background()
	flow, _ := beginGoogle(t, s, "signup-race", "same@gmail.com", true)
	var signup Result
	var google GoogleResult
	var signupErr, googleErr error
	gate := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-gate
		signup, signupErr = s.Signup(ctx, SignupInput{"same@gmail.com", "Same", "oldpassword1!"}, "test")
	}()
	go func() {
		defer wg.Done()
		<-gate
		google, googleErr = s.CompleteGoogle(ctx, flow.Token, "web", "Same", "newpassword2!", "test")
	}()
	close(gate)
	wg.Wait()
	if googleErr != nil || google.Account == nil {
		t.Fatal(googleErr)
	}
	if signupErr != nil && !errors.Is(signupErr, ErrEmailTaken) {
		t.Fatal(signupErr)
	}
	if signupErr == nil {
		if _, err := s.Authenticate(ctx, signup.Token); !errors.Is(err, ErrSession) {
			t.Fatal("signup race left old session", err)
		}
	}
	if _, err := s.Login(ctx, "Same", "newpassword2!", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Login(ctx, "Same", "oldpassword1!", "test"); !errors.Is(err, ErrCredentials) {
		t.Fatal("old password retained", err)
	}
}

func TestGoogleOnboardingConflictsAndRollback(t *testing.T) {
	s := googleTestService(t)
	ctx := context.Background()
	existing := signupForRecovery(t, s, "Occupied")
	flow, _ := beginGoogle(t, s, "retry-subject", "retry@gmail.com", true)
	if _, err := s.CompleteGoogle(ctx, flow.Token, "web", "oCCUPIED", "password1!", "test"); !errors.Is(err, ErrUsernameTaken) {
		t.Fatal("username conflict missing", err)
	}
	state, err := s.GoogleContext(ctx, flow.Token, "web")
	if err != nil || state.Status != "onboarding" {
		t.Fatal("conflict consumed flow", state, err)
	}
	var users, identities int
	if err = s.repo.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM external_identities)`).Scan(&users, &identities); err != nil || users != 1 || identities != 0 {
		t.Fatal("conflict leaked records", users, identities, err)
	}
	// Simulate a late persistence failure after account creation, binding and session insertion.
	if _, err = s.repo.pool.Exec(ctx, `ALTER TABLE auth_google_flows ADD CONSTRAINT reject_completion CHECK (status<>'consumed')`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteGoogle(ctx, flow.Token, "web", "Retry", "password1!", "test"); err == nil {
		t.Fatal("forced persistence failure did not fail")
	}
	if err = s.repo.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM external_identities)`).Scan(&users, &identities); err != nil || users != 1 || identities != 0 {
		t.Fatal("completion transaction leaked records", users, identities, err)
	}
	if _, err = s.repo.pool.Exec(ctx, `ALTER TABLE auth_google_flows DROP CONSTRAINT reject_completion`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CompleteGoogle(ctx, flow.Token, "web", "Retry", "password1!", "test"); err != nil {
		t.Fatal("retry failed", err)
	}
	// If the original registration verifies during onboarding, its password must survive.
	pendingFlow, _ := beginGoogle(t, s, "occupied-subject", existing.Session.User.Email, true)
	if err = s.ConfirmVerification(ctx, queuedRecoveryToken(t, s, existing.Session.User.ID, "verification"), "test"); err != nil {
		t.Fatal(err)
	}
	linked, err := s.CompleteGoogle(ctx, pendingFlow.Token, "web", "Ignored", "newpassword2!", "test")
	if err != nil || linked.Account == nil || linked.Account.Session.User.Username != "Occupied" {
		t.Fatal(linked, err)
	}
	if _, err = s.Login(ctx, "Occupied", "password1!", "test"); err != nil {
		t.Fatal("newly active password replaced", err)
	}
	if _, err = s.Login(ctx, "Occupied", "newpassword2!", "test"); !errors.Is(err, ErrCredentials) {
		t.Fatal("onboarding replaced active password", err)
	}
}

func TestGoogleThirdPartyExistingAccount(t *testing.T) {
	s := googleTestService(t)
	ctx := context.Background()
	existing := signupForRecovery(t, s, "ThirdParty")
	if err := s.ConfirmVerification(ctx, queuedRecoveryToken(t, s, existing.Session.User.ID, "verification"), "test"); err != nil {
		t.Fatal(err)
	}
	flow, pending := beginGoogle(t, s, "third-party-subject", existing.Session.User.Email, false)
	if pending.Account != nil || pending.Context.Status != "email_verification_required" {
		t.Fatal("missing fresh ownership challenge")
	}
	if err := s.ConfirmGoogleEmail(ctx, googleEmailToken(t, s, flow), "test"); err != nil {
		t.Fatal(err)
	}
	ready, err := s.GoogleContext(ctx, flow.Token, "web")
	if err != nil || ready.Status != "ready" {
		t.Fatal(ready, err)
	}
	linked, err := s.ExchangeGoogle(ctx, flow.Token, "web", "test")
	if err != nil || linked.Account == nil || linked.Account.Session.User.ID != existing.Session.User.ID {
		t.Fatal(linked, err)
	}
	if _, err = s.Login(ctx, "ThirdParty", "password1!", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ExchangeGoogle(ctx, flow.Token, "web", "test"); !errors.Is(err, ErrInvalidFlow) {
		t.Fatal("exchange replay", err)
	}
}

func TestGoogleChallengeQuotasExpiryAndCleanup(t *testing.T) {
	s := googleTestService(t)
	ctx := context.Background()
	s.recovery.config.Policy.MailPerAccount = 1
	flow, _ := beginGoogle(t, s, "quota-subject", "quota@example.com", false)
	token := googleEmailToken(t, s, flow)
	// A password signup cannot bypass the combined recipient mail quota.
	if _, err := s.Signup(ctx, SignupInput{"QUOTA@example.com", "Quota", "password1!"}, "test"); !errors.Is(err, ErrRateLimited) {
		t.Fatal("recipient quota bypass", err)
	}
	if _, err := s.repo.pool.Exec(ctx, `UPDATE auth_google_flows SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, tokenHash(flow.Token)); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfirmGoogleEmail(ctx, token, "test"); !errors.Is(err, ErrTokenExpired) {
		t.Fatal("expired flow challenge accepted", err)
	}
	sender := &maildelivery.Fake{}
	if worked, err := s.DeliverOne(ctx, sender); err != nil || !worked || len(sender.Messages()) != 0 {
		t.Fatal("expired challenge delivered", worked, err)
	}
	if _, err := s.repo.pool.Exec(ctx, `UPDATE auth_google_flows SET expires_at=now()-interval '8 days' WHERE token_hash=$1`, tokenHash(flow.Token)); err != nil {
		t.Fatal(err)
	}
	if err := s.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	var flows, tokens, outbox int
	if err := s.repo.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM auth_google_flows),(SELECT count(*) FROM auth_tokens),(SELECT count(*) FROM auth_mail_outbox)`).Scan(&flows, &tokens, &outbox); err != nil || flows+tokens+outbox != 0 {
		t.Fatal("expired flow cleanup failed", flows, tokens, outbox, err)
	}
	disabled := accountTestService(t, s.repo.pool, DefaultPolicy())
	if _, err := disabled.StartGoogle(ctx, "web", "test"); !errors.Is(err, ErrGoogleDisabled) {
		t.Fatal("unconfigured Google accepted", err)
	}
}

func TestGoogleConcurrentProofAndProviderCollision(t *testing.T) {
	for _, sameFlow := range []bool{true, false} {
		t.Run(map[bool]string{true: "proof replay", false: "different subjects"}[sameFlow], func(t *testing.T) {
			s := googleTestService(t)
			ctx := context.Background()
			user := signupForRecovery(t, s, "ProofRace")
			if err := s.ConfirmVerification(ctx, queuedRecoveryToken(t, s, user.Session.User.ID, "verification"), "test"); err != nil {
				t.Fatal(err)
			}
			first, err := s.StartGoogle(ctx, "web", "test")
			if err != nil {
				t.Fatal(err)
			}
			second := first
			if !sameFlow {
				second, err = s.StartGoogle(ctx, "web", "test")
				if err != nil {
					t.Fatal(err)
				}
			}
			proofs := []string{googleProof(t, first, "subject-a", user.Session.User.Email, true), googleProof(t, second, "subject-a", user.Session.User.Email, true)}
			if !sameFlow {
				proofs[1] = googleProof(t, second, "subject-b", user.Session.User.Email, true)
			}
			gate := make(chan struct{})
			results := make(chan error, 2)
			var wg sync.WaitGroup
			for i, flow := range []GoogleStart{first, second} {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-gate
					_, err := s.ProveGoogle(ctx, flow.Token, "web", proofs[i], "test")
					results <- err
				}()
			}
			close(gate)
			wg.Wait()
			close(results)
			successes := 0
			expectedError := ErrIdentityConflict
			if sameFlow {
				expectedError = ErrInvalidFlow
			}
			for err := range results {
				if err == nil {
					successes++
				} else if !errors.Is(err, expectedError) {
					t.Fatal(err)
				}
			}
			if successes != 1 {
				t.Fatal("concurrent proof/collision did not serialize", successes)
			}
			var identities int
			if err := s.repo.pool.QueryRow(ctx, `SELECT count(*) FROM external_identities`).Scan(&identities); err != nil || identities != 1 {
				t.Fatal(identities, err)
			}
		})
	}
}
