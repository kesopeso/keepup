"use client";

import { useEffect, useRef, useState, useSyncExternalStore, type FormEvent } from "react";
import { Brand } from "../components/ui";
import { authRequest, AuthError, authMessage, passwordError, passwordHint, usernameError, type AccountSession, type GoogleContext, type GoogleResult } from "../../lib/auth-api";
import { checkedDestination, rememberDestination, safeDestination, savedDestination } from "../../lib/auth-destination";
import { currentSession, refreshAccountSession, setAccountSession, subscribeSession, watchAccountSession } from "../../lib/account-session";
import { loadGoogleIdentity } from "../../lib/google-signin";
import styles from "./auth.module.css";

export type AuthScreenName = "login" | "signup" | "verify" | "forgot" | "reset" | "google-verify" | "google" | "account";
const titles: Record<AuthScreenName, string> = { login: "Welcome back", signup: "Create your account", verify: "Check your email", forgot: "Forgot your password?", reset: "Choose a new password", "google-verify": "Verify your Google email", google: "Make it yours", account: "Your account" };
const serverSession = () => null;

function PasswordField({ name, label, newPassword = false, error, onEdit }: { name: string; label: string; newPassword?: boolean; error?: string; onEdit: () => void }) {
  const [visible, setVisible] = useState(false);
  return <div className={styles.field}>
    <label htmlFor={name}>{label}</label>
    <div className={styles.password}>
      <input id={name} name={name} type={visible ? "text" : "password"} autoComplete={newPassword ? "new-password" : "current-password"} required aria-invalid={!!error} aria-describedby={error ? `${name}-error` : newPassword ? "password-hint" : undefined} onChange={onEdit} />
      <button type="button" aria-label={`${visible ? "Hide" : "Show"} ${label.toLowerCase()}`} aria-pressed={visible} onClick={() => setVisible(!visible)}>{visible ? "Hide" : "Show"}</button>
    </div>
    {error && <p className={styles.error} id={`${name}-error`}>{error}</p>}
  </div>;
}

export default function AuthScreen({ screen }: { screen: AuthScreenName }) {
  const session = useSyncExternalStore(subscribeSession, currentSession, serverSession);
  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [destination, setDestination] = useState("/my-routes");
  const [token, setToken] = useState("");
  const [linkMode, setLinkMode] = useState(false);
  const [done, setDone] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [google, setGoogle] = useState<GoogleContext | null>(null);
  const [googleStarted, setGoogleStarted] = useState(false);
  const googleButton = useRef<HTMLDivElement>(null);
  const operation = useRef(false);
  const alive = useRef(true);
  const googleAttempt = useRef(0);
  const capturedLink = useRef(false);
  const href = (path: string) => `${path}?next=${encodeURIComponent(destination)}`;

  useEffect(() => {
    alive.current = true;
    const url = new URL(window.location.href);
    if (url.searchParams.has("next")) rememberDestination(url.searchParams.get("next"));
    setDestination(safeDestination(url.searchParams.get("next")) ?? savedDestination());
    const captureLink = () => {
      const link = new URL(window.location.href);
      if (capturedLink.current && !link.hash) return;
      capturedLink.current = true;
      const fragment = new URLSearchParams(link.hash.slice(1));
      const linkToken = fragment.get("token") ?? "";
      setLinkMode(!!link.hash);
      setToken(/^[A-Za-z0-9_-]{43}$/.test(linkToken) ? linkToken : "");
      setDone(false); setError(""); setNotice(""); setFields({});
      if (link.hash) window.history.replaceState(null, "", link.pathname + link.search);
    };
    captureLink();
    window.addEventListener("hashchange", captureLink);
    if (url.searchParams.get("reset") === "done") setNotice("Password reset. Sign in with your new password.");
    void refreshAccountSession().catch((err) => { if (alive.current) setError(authMessage(err)); }).finally(() => { if (alive.current) setReady(true); });
    const stop = watchAccountSession((err) => setError(authMessage(err)));
    return () => { alive.current = false; googleAttempt.current++; stop(); window.removeEventListener("hashchange", captureLink); };
  }, []);

  useEffect(() => {
    if (!cooldown) return;
    const timer = window.setInterval(() => setCooldown((value) => Math.max(0, value - 1)), 1000);
    return () => clearInterval(timer);
  }, [cooldown > 0]);

  useEffect(() => {
    if (!ready) return;
    if (screen === "account" && !session) {
      window.location.replace(`/auth/login?next=${encodeURIComponent("/account")}`);
    }
    if (screen === "account" && session && !session.user.verified) window.location.replace(href("/auth/verify"));
  }, [ready, session, screen]);

  // The initiating Google tab resumes after an email opened on another device or a restart.
  useEffect(() => {
    if (screen !== "google") return;
    let stopped = false;
    let polling = false;
    const check = async () => {
      if (stopped || polling || operation.current || document.visibilityState === "hidden") return;
      polling = true;
      try {
        const context = await authRequest<GoogleContext>("google/context");
        if (!stopped) { setGoogle(context); setError(""); }
      } catch (err) {
        if (!stopped) {
          setError(authMessage(err));
          if (err instanceof AuthError && ["invalid_flow", "flow_expired"].includes(err.code)) setGoogle(null);
        }
      }
      finally { polling = false; }
    };
    void check();
    const timer = window.setInterval(check, 5000);
    window.addEventListener("focus", check);
    return () => { stopped = true; clearInterval(timer); window.removeEventListener("focus", check); };
  }, [screen]);

  async function run(action: () => Promise<void>) {
    if (operation.current) return;
    operation.current = true; setBusy(true); setError(""); setNotice(""); setFields({});
    try { await action(); }
    catch (err) {
      if (!alive.current) return;
      setError(authMessage(err));
      if (err instanceof AuthError) {
        if (err.retryAfter) setCooldown(Math.ceil(err.retryAfter));
        if (err.code === "email_taken") setFields({ email: authMessage(err) });
        if (err.code === "username_taken") setFields({ username: authMessage(err) });
        if (screen === "account" && err.code === "invalid_credentials") setFields({ currentPassword: "Your current password is incorrect." });
        if (err.code === "session_expired") setAccountSession(null);
      }
    } finally { operation.current = false; if (alive.current) setBusy(false); }
  }

  async function resume() {
    await run(async () => {
      const active = await refreshAccountSession();
      if (!active) throw new AuthError("session_expired");
      if (!active.user.verified) { setNotice("Your email has not been verified yet. Open the latest email, then try again."); return; }
      let result;
      try { result = await checkedDestination(destination); }
      catch (err) {
        if (err instanceof Error && err.message === "session_expired") { setAccountSession(null); throw new AuthError("session_expired"); }
        throw new AuthError("offline");
      }
      if (result.unavailable) { setDestination("/my-routes"); rememberDestination("/my-routes"); setNotice("That route is no longer available. Continue to My routes."); return; }
      rememberDestination(null);
      window.location.assign(result.path);
    });
  }

  function acceptSession(value: AccountSession) {
    setAccountSession(value);
    // Keep the destination in the URL as well as storage across the activation step.
    window.location.assign(href("/auth/verify"));
  }
  function acceptGoogle(value: GoogleResult) {
    if (value.status === "authenticated") acceptSession(value);
    else { setGoogle(value); window.location.assign(href("/auth/google")); }
  }
  async function beginGoogle() {
    await run(async () => {
      const config = await fetch("/auth/config", { cache: "no-store" }).then((response) => response.json());
      if (!config.googleClientId) throw new AuthError("google_unavailable");
      const identity = await loadGoogleIdentity().catch(() => { throw new AuthError("google_unavailable"); });
      const flow = await authRequest<{ nonce: string; expiresAt: string }>("google/start", {});
      const attempt = ++googleAttempt.current;
      setGoogleStarted(true);
      identity.initialize({ client_id: config.googleClientId, nonce: flow.nonce, auto_select: false, ux_mode: "popup", callback: (response) => {
        if (!alive.current || googleAttempt.current !== attempt) return;
        void run(async () => acceptGoogle(await authRequest<GoogleResult>("google/proof", { idToken: response.credential })));
      } });
      if (googleButton.current) identity.renderButton(googleButton.current, { theme: "filled_black", size: "large", text: "continue_with", width: Math.min(360, googleButton.current.parentElement?.clientWidth ?? 320) });
    });
  }
  function validate(data: FormData): boolean {
    const next: Record<string, string> = {};
    const password = String(data.get("password") ?? "");
    if (screen === "signup" || (screen === "google" && google?.status === "onboarding")) {
      const message = usernameError(String(data.get("username") ?? "")); if (message) next.username = message;
    }
    if (["signup", "reset", "account", "google"].includes(screen)) {
      const message = passwordError(password); if (message) next.password = message;
      if (password !== data.get("confirmation")) next.confirmation = "Passwords must match. Enter the same password in both fields.";
    }
    if (Object.keys(next).length) {
      setFields(next); setError("Check the highlighted fields.");
      document.getElementById(Object.keys(next)[0])?.focus();
      return false;
    }
    return true;
  }
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    if (!validate(data)) return;
    const text = (key: string) => String(data.get(key) ?? "");
    void run(async () => {
      if (screen === "login") acceptSession(await authRequest<AccountSession>("login", { identifier: text("identifier"), password: text("password") }));
      if (screen === "signup") acceptSession(await authRequest<AccountSession>("signup", { email: text("email").trim(), username: text("username"), password: text("password") }));
      if (screen === "forgot") {
        await authRequest("password/forgot", { identifier: text("identifier") }); setDone(true);
      }
      if (screen === "reset") {
        await authRequest("password/reset", { token, password: text("password") });
        setToken(""); setAccountSession(null); form.reset();
        window.location.replace(href("/auth/login") + "&reset=done");
      }
      if (screen === "account") {
        setAccountSession(await authRequest<AccountSession>("password/change", { currentPassword: text("currentPassword"), password: text("password") }));
        form.reset(); setNotice("Password changed. Your other devices have been logged out.");
      }
      if (screen === "google") acceptGoogle(await authRequest<GoogleResult>("google/complete", { username: text("username"), password: text("password") }));
    });
  }
  const edit = () => { setFields({}); setError(""); };
  const input = (name: string, label: string, autoComplete: string, type = "text", value?: string) => <div className={styles.field}>
    <label htmlFor={name}>{label}</label>
    <input id={name} name={name} type={type} autoComplete={autoComplete} autoCapitalize="none" spellCheck={false} required readOnly={value !== undefined} defaultValue={value} aria-invalid={!!fields[name]} aria-describedby={fields[name] ? `${name}-error` : name === "username" ? "username-hint" : undefined} onChange={edit} />
    {fields[name] && <p className={styles.error} id={`${name}-error`}>{fields[name]}</p>}
  </div>;
  const passwords = <>
    <PasswordField name="password" label={screen === "account" || screen === "reset" ? "New password" : "Password"} newPassword error={fields.password} onEdit={edit} />
    <p id="password-hint" className={styles.small}>{passwordHint}</p>
    <PasswordField name="confirmation" label="Confirm password" newPassword error={fields.confirmation} onEdit={edit} />
  </>;
  const googleChoice = <>
    <div className={styles.divider}>or</div>
    <button type="button" className={styles.google} disabled={busy || !ready || cooldown > 0} onClick={beginGoogle}><GoogleIcon />Continue with Google</button>
    <div ref={googleButton} className={styles.googleSlot} hidden={!googleStarted} />
    {googleStarted && <><p className={styles.small}>Choose the Google button above to continue. If you close the window, you can try again or cancel.</p><button type="button" onClick={() => { googleAttempt.current++; window.google?.accounts.id.cancel(); setGoogleStarted(false); googleButton.current?.replaceChildren(); }}>Cancel Google sign-in</button></>}
  </>;
  const authenticated = session && session.user.verified;
  const linkPage = (screen === "verify" && linkMode) || screen === "google-verify";
  return <main className={styles.shell}>
    <section className={styles.card} aria-labelledby="auth-title">
      <Brand />
      <h1 id="auth-title">{titles[screen]}</h1>
      {screen === "reset" && <p className={styles.warning}>This logs out all devices and stops location sharing on them.</p>}
      {destination.startsWith("/routes/") && <p className={styles.hint}>Your route will be waiting after you sign in and verify your email.</p>}
      {!ready && <p role="status">Checking your session…</p>}
      {notice && <p className={styles.hint} role="status">{notice}</p>}
      {error && <p className={styles.error} role="alert">{error}</p>}
      {cooldown > 0 && <p role="status" className={styles.small}>Try again in {cooldown} {cooldown === 1 ? "second" : "seconds"}.</p>}
      {ready && <>
        {(["login", "signup"].includes(screen) && session) ? <>
          <p>Signed in as {session.user.username}.</p>
          <button disabled={busy} onClick={() => window.location.assign(href("/auth/verify"))}>Continue</button>
          <button disabled={busy} onClick={() => logout(false)}>Use another account</button>
        </> : ["login", "signup", "forgot", "reset"].includes(screen) && !(screen === "forgot" && done) && <form onSubmit={submit}>
          <fieldset disabled={busy || cooldown > 0}>
            {screen === "signup" && <><p>Keep your routes with you.</p>{input("email", "Email", "email", "email")}{input("username", "Username", "username")}<p id="username-hint" className={styles.small}>3–24 letters, numbers, hyphens (-), or underscores (_). Email and username cannot be changed.</p>{passwords}</>}
            {screen === "login" && <>{input("identifier", "Email or username", "username")}<PasswordField name="password" label="Password" error={fields.password} onEdit={edit} /></>}
            {screen === "forgot" && <><p>Enter your email or username. If it matches an account, we will send a reset link.</p>{input("identifier", "Email or username", "username")}</>}
            {screen === "reset" && (token ? passwords : <p>Open the latest password reset link from your email. If you reloaded this page, reopen the email link.</p>)}
            <button className={styles.primary} type="submit" disabled={screen === "reset" && !token}>{screen === "signup" && <AccountIcon />}{busy ? "Please wait…" : screen === "login" ? "Sign in" : screen === "signup" ? "Create account" : screen === "forgot" ? "Send reset link" : "Reset password"}</button>
          </fieldset>
        </form>}
        {screen === "forgot" && done && <p className={styles.hint} role="status">If an account matches, a reset link is on its way. Check your inbox and spam folder.</p>}
        {linkPage && <>
          {done ? <p className={styles.hint}>Email verified. Return to the device where you started, or sign in here.</p> : token ? <><p>Confirm to verify this email. Opening this link does not sign you in.</p><button className={styles.primary} disabled={busy || cooldown > 0} onClick={() => run(async () => {
            await authRequest(screen === "google-verify" ? "google/email/confirm" : "verification/confirm", { token });
            setToken(""); setDone(true); await refreshAccountSession();
          })}>Verify email</button></> : <p>Open the latest verification link from your email. If you reloaded this page, reopen the email link.</p>}
          {screen === "google-verify" && <a href={href("/auth/google")}>Return to Google setup</a>}
        </>}
        {screen === "verify" && (!linkMode || done) && <>
          {authenticated ? <><p>Email verified. You are signed in as {session.user.username}.</p><button className={styles.primary} disabled={busy} onClick={resume}>Continue{destination === "/my-routes" ? " to My routes" : ""}</button></> : session ? <>
            <p>Open the verification link sent to {session.user.email} to activate your account.</p>
            <button className={styles.primary} disabled={busy || cooldown > 0} onClick={resume}>I have verified my email</button>
          </> : <p>Sign in to check activation or request a new verification email.</p>}
        </>}
        {screen === "verify" && session && !session.user.verified && <>
          <button disabled={busy || cooldown > 0} onClick={() => run(async () => { await authRequest("verification/resend", {}); setCooldown(60); setNotice("Verification requested. Use the latest email. Delivery may take a few minutes."); })}>Resend verification email</button>
          <button disabled={busy} onClick={() => logout(false)}>Use another account</button>
        </>}
        {screen === "google" && <>
          {google?.status === "onboarding" && <form onSubmit={submit}><p className={styles.hint}>{google.email} · Email verified</p><fieldset disabled={busy || cooldown > 0}>
            {input("username", "Username", "username", "text", google.username)}<p id="username-hint" className={styles.small}>3–24 letters, numbers, hyphens (-), or underscores (_). Email and username cannot be changed.</p>
            {passwords}<button className={styles.primary} type="submit"><AccountIcon />Complete signup</button>
          </fieldset></form>}
          {google?.status === "email_verification_required" && <><p>Check {google.email ?? "your email"} for a KeepUp verification link. Return here after confirming it.</p><button disabled={busy || cooldown > 0} onClick={() => run(async () => { await authRequest("google/email/resend", {}); setCooldown(60); setNotice("Verification requested. Use the latest email."); })}>Resend verification email</button></>}
          {google?.status === "ready" && <button className={styles.primary} disabled={busy} onClick={() => run(async () => acceptGoogle(await authRequest<GoogleResult>("google/exchange", {})))}>Continue with verified email</button>}
          <a href={href("/auth/login")}>Cancel and return to sign in</a>
        </>}
        {screen === "account" && authenticated && <>
          <dl><dt>Username</dt><dd>{session.user.username}</dd><dt>Email</dt><dd>{session.user.email}</dd></dl>
          <p className={styles.small}>Email and username cannot be changed.</p>
          <form key={session.user.id} onSubmit={submit}><fieldset disabled={busy || cooldown > 0}>
            <PasswordField name="currentPassword" label="Current password" error={fields.currentPassword} onEdit={edit} />{passwords}
            <p className={styles.small}>Changing your password logs out your other devices and stops their sharing.</p>
            <button className={styles.primary} type="submit">Change password</button>
          </fieldset></form>
          <button disabled={busy} onClick={() => logout(false)}>Log out</button>
          <button disabled={busy} onClick={() => logout(true)}>Log out all devices</button>
          <a href="/my-routes">My routes</a>
        </>}
        {["login", "signup"].includes(screen) && !session && googleChoice}
        <nav className={styles.links} aria-label="Account navigation">
          {screen === "login" && <a href={href("/auth/forgot-password")}>Forgot password?</a>}
          {screen === "reset" && <a href={href("/auth/forgot-password")}>Request a new reset link</a>}
          {screen !== "signup" && screen !== "account" && <a href={href("/auth/signup")}><AccountIcon />Create an account</a>}
          {screen !== "login" && screen !== "account" && <a href={href("/auth/login")}>Back to sign in</a>}
        </nav>
      </>}
    </section>
  </main>;

  function logout(all: boolean) {
    void run(async () => {
      try { await authRequest(all ? "logout-all" : "logout", {}); }
      catch (err) { if (!(err instanceof AuthError && err.code === "session_expired")) throw err; }
      setAccountSession(null); window.google?.accounts.id.disableAutoSelect();
      window.location.replace(href("/auth/login"));
    });
  }
}
function AccountIcon() { return <svg className={styles.icon} viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8"><circle cx="9" cy="7" r="4" /><path d="M2 21v-3a7 7 0 0 1 14 0v3M20 7v6m-3-3h6" /></svg>; }
function GoogleIcon() { return <svg className={styles.icon} viewBox="0 0 24 24" aria-hidden="true"><path fill="#4285F4" d="M21.6 12.2c0-.7-.1-1.4-.2-2.1H12v4h5.4a4.6 4.6 0 0 1-2 3v2.6h3.3c1.9-1.8 2.9-4.4 2.9-7.5Z" /><path fill="#34A853" d="M12 22c2.7 0 5-.9 6.7-2.3l-3.3-2.6c-.9.6-2.1.9-3.4.9-2.6 0-4.8-1.8-5.6-4.1H3v2.7A10 10 0 0 0 12 22Z" /><path fill="#FBBC05" d="M6.4 13.9a6 6 0 0 1 0-3.8V7.4H3a10 10 0 0 0 0 9.2l3.4-2.7Z" /><path fill="#EA4335" d="M12 6c1.5 0 2.9.5 3.9 1.5l2.9-2.9A9.6 9.6 0 0 0 12 2a10 10 0 0 0-9 5.4l3.4 2.7A6 6 0 0 1 12 6Z" /></svg>; }
