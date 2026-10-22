import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { ActivityIndicator, AppState, BackHandler, Image, Pressable, TextInput, View } from 'react-native';
import { BrandText as Text } from '../components/Brand';
import { ActionButton, ErrorMessage, Field, Screen, styles } from '../components/ui';
import { getApiBaseUrl } from '../api/config';
import { AccountError, errorMessage } from './api';
import { authOrigin, parseAuthLink, passwordError, usernameError } from './contracts';
import type { LinkIntent } from './contracts';
import type { AccountController } from './controller';
import { nativeAccountController, pickGoogleProof, observeAccountLinks } from './native';
import { AccountRoutes } from './AccountRoutes';
import { freshList } from './route-api';

type Page = 'login' | 'signup' | 'forgot' | 'reset' | 'verify' | 'google-verify' | 'change' | 'account';
export function AccountApp() {
  const [controller, setController] = useState<AccountController | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let live = true;
    void nativeAccountController(getApiBaseUrl()).then((value) => { if (live) setController(value); })
      .catch(() => { if (live) setError('Could not open secure account storage. Please try again.'); });
    return () => { live = false; };
  }, [attempt]);
  if (!controller) return <Screen><Text style={styles.title}>Opening KeepUp</Text><ErrorMessage message={error} />
    {error ? <ActionButton label="Retry" onPress={() => { setError(null); setAttempt(attempt + 1); }} /> : <ActivityIndicator accessibilityLabel="Opening secure account storage" />}</Screen>;
  return <AccountScreens controller={controller} />;
}

export function AccountScreens({ controller }: { controller: AccountController }) {
  const state = useSyncExternalStore(controller.subscribe, controller.snapshot);
  const [page, setPage] = useState<Page>('login');
  const [link, setLink] = useState<Exclude<LinkIntent, { screen: 'destination' }> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [googlePicker, setGooglePicker] = useState(false);
  const [blockedUntil, setBlockedUntil] = useState(0);
  const [now, setNow] = useState(Date.now());
  const [linkRevision, setLinkRevision] = useState(0);
  const alive = useRef(true);
  const appState = useRef(AppState.currentState);
  const refreshBusy = useRef(false);
  const listCache = useRef(freshList());
  const cacheUser = useRef<string | null>(null);
  const [settings, setSettings] = useState(false);
  const identity = state.user?.id ?? null;
  if (cacheUser.current !== identity) { cacheUser.current = identity; listCache.current = freshList(); }
  useEffect(() => { setSettings(false); }, [identity]);
  const cooldown = Math.max(0, Math.ceil((blockedUntil - now) / 1000));
  const locked = busy || cooldown > 0;
  const google = state.google;

  function navigate(next: Page) { setPage(next); setError(null); setNotice(null); setLink(null); setLinkRevision((v) => v + 1); }
  async function run(work: () => Promise<unknown>, message?: string) {
    if (busy) return;
    setBusy(true); setError(null); setNotice(null);
    try { await work(); if (alive.current && message) setNotice(message); }
    catch (reason) {
      if (alive.current) {
        setError(errorMessage(reason));
        if (reason instanceof AccountError && reason.retryAfter) setBlockedUntil(Date.now() + reason.retryAfter * 1000);
      }
      return reason;
    } finally { if (alive.current) setBusy(false); }
  }
  useEffect(() => {
    alive.current = true;
    void controller.initialize().catch((reason) => { if (alive.current) setError(errorMessage(reason)); });
    return () => { alive.current = false; };
  }, [controller]);
  useEffect(() => {
    function incoming(url: string) {
      let intent: LinkIntent | null = null;
      try { intent = parseAuthLink(url, authOrigin(process.env.EXPO_PUBLIC_AUTH_LINK_ORIGIN)); } catch { /* invalid deployment config */ }
      if (!intent) { setLink(null); setPage('login'); setLinkRevision((v) => v + 1); setError('This KeepUp link is invalid. Request a new link or sign in.'); return; }
      setError(null); setNotice(null); setLinkRevision((v) => v + 1);
      if (intent.screen === 'destination') {
        void controller.setDestination(intent.destination).catch((reason) => setError(errorMessage(reason)));
        setLink(null); setSettings(false); setPage('login');
      } else {
        setLink(intent); setPage(intent.screen);
      }
      // Keep no URL, fragment or token in navigation persistence or analytics.
    }
    try { return observeAccountLinks(incoming); }
    catch { setError("Rebuild the Android app to enable account links."); }
  }, [controller]);
  useEffect(() => {
    async function refresh() {
      if (appState.current !== 'active' || refreshBusy.current) return;
      refreshBusy.current = true;
      try { await controller.refresh(); }
      catch (reason) {
        if (alive.current && reason instanceof AccountError && ['session_expired', 'flow_expired', 'invalid_flow'].includes(reason.code)) setError(errorMessage(reason));
      } finally { refreshBusy.current = false; }
    }
    const subscription = AppState.addEventListener('change', (next) => { appState.current = next; if (next === 'active') void refresh(); });
    const timer = setInterval(() => { setNow(Date.now()); void refresh(); }, 5000);
    return () => { subscription.remove(); clearInterval(timer); };
  }, [controller]);
  useEffect(() => {
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
      if (busy || googlePicker) return true;
      if (link || page !== 'login' && page !== 'account' || google) {
        if (google) void controller.cancelGoogle();
        navigate(state.user ? 'account' : 'login'); return true;
      }
      return false;
    });
    return () => subscription.remove();
  }, [busy, googlePicker, link, page, google, state.user]);
  useEffect(() => {
    if (!state.user?.verified) return;
    void controller.checkDestination().then((target) => {
      if (target === '/my-routes' && state.destination !== target && alive.current) setNotice('That route is unavailable. Your account is ready to use.');
    }).catch((reason) => { if (alive.current) setError(errorMessage(reason)); });
  }, [controller, state.user?.id, state.user?.verified, state.destination]);

  if (!state.ready) return <Screen><Text style={styles.title}>Restoring account access</Text><ErrorMessage message={error} />{error ? <ActionButton label="Retry restoring access" disabled={busy} onPress={() => void run(() => controller.initialize())} /> : <ActivityIndicator accessibilityLabel="Checking account session" />}</Screen>;
  if (state.saving) return <Screen><Text style={styles.title}>Save account access</Text><Text style={styles.text}>KeepUp needs to save this session securely before you continue.</Text>
    <ErrorMessage message={error} /><ActionButton label="Retry saving access" disabled={busy} onPress={() => void run(() => controller.retrySave())} /></Screen>;

  const linkScreen = link && ['verify', 'reset', 'google-verify'].includes(page);
  if (state.user?.verified && !linkScreen && !settings && page !== 'change') {
    return <AccountRoutes key={state.user.id} controller={controller} cache={listCache} accountNotice={notice} onAccount={() => { setSettings(true); navigate('account'); }} />;
  }
  let shown: Page | 'pending' | 'google-onboarding' | 'google-pending' = page;
  if (!linkScreen) {
    if (google?.status === 'onboarding') shown = 'google-onboarding';
    else if (google) shown = 'google-pending';
    else if (state.user && !state.user.verified) shown = 'pending';
    else if (state.user?.verified && page !== 'change') shown = 'account';
    else if (!state.user && ['change', 'account'].includes(page)) shown = 'login';
  }
  const titles = { login: 'Welcome back', signup: 'Create your account', forgot: 'Forgot your password?', reset: 'Reset your password',
    verify: 'Verify your email', 'google-verify': 'Confirm your Google email', change: 'Change password', account: 'Account settings',
    pending: 'Check your email', 'google-onboarding': 'Finish creating your account', 'google-pending': 'Verify your Google email' };
  return <Screen>
    <Text style={styles.title}>{titles[shown]}</Text>
    {shown === 'reset' && <Text style={styles.text}>Resetting your password signs you out on all devices. It does not verify your email.</Text>}
    <ErrorMessage message={error} />
    {notice && <Text style={styles.text} accessibilityLiveRegion="polite">{notice}</Text>}
    {cooldown > 0 && <Text style={styles.text} accessibilityLiveRegion="polite">Try again in {cooldown} seconds.</Text>}
    {shown === 'login' && <>
      <AccountForm key={`login-${linkRevision}`} kind="login" locked={locked} onSubmit={(v) => run(() => controller.login(v.identifier, v.password))} />
      <ActionButton label="Forgot password?" secondary disabled={busy} onPress={() => navigate('forgot')} />
      <GoogleButton locked={locked} onPress={() => { setGooglePicker(true); void run(() => controller.startGoogle(pickGoogleProof)).finally(() => setGooglePicker(false)); }} />
      <ActionButton label="Create an account" secondary disabled={busy} onPress={() => navigate('signup')} />
    </>}
    {shown === 'signup' && <>
      <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={{ alignSelf: 'center', width: 52, height: 50 }}><View style={{ width: 18, height: 18, borderRadius: 9, borderWidth: 2, borderColor: '#22c55e', marginLeft: 10 }} /><View style={{ width: 38, height: 24, borderTopLeftRadius: 20, borderTopRightRadius: 20, borderWidth: 2, borderColor: '#22c55e', marginTop: 4 }} /><Text style={{ color: '#22c55e', fontSize: 25, position: 'absolute', right: 0, top: 0 }}>+</Text></View>
      <AccountForm kind="signup" locked={locked} onSubmit={(v) => run(() => controller.signup(v.email, v.username, v.password))} />
      <GoogleButton locked={locked} onPress={() => { setGooglePicker(true); void run(() => controller.startGoogle(pickGoogleProof)).finally(() => setGooglePicker(false)); }} />
      <ActionButton label="Already have an account? Sign in" secondary disabled={busy} onPress={() => navigate('login')} />
    </>}
    {shown === 'forgot' && <><Text style={styles.text}>Enter your email or username. If an account matches, we'll send a reset link.</Text>
      <AccountForm kind="forgot" locked={locked} onSubmit={(v) => run(() => controller.forgot(v.identifier), 'If an account matches, a reset email is on its way.')} />
      <ActionButton label="Back to sign in" secondary disabled={busy} onPress={() => navigate('login')} /></>}
    {shown === 'reset' && <><AccountForm key={`reset-${linkRevision}`} kind="reset" locked={locked || !link} onSubmit={(v) => run(async () => {
      if (!link) throw new AccountError('invalid_token'); await controller.reset(link.token, v.password); navigate('login');
    }, 'Your password has been reset. Sign in with your new password.')} />
      <ActionButton label="Request another reset link" secondary disabled={busy} onPress={() => navigate('forgot')} /></>}
    {(shown === 'verify' || shown === 'google-verify') && <>
      <Text style={styles.text}>Confirm this email link. This does not sign in this device.</Text>
      <ActionButton label="Confirm email" disabled={locked || !link} onPress={() => void run(async () => {
        if (!link) throw new AccountError('invalid_token'); await controller.confirm(link.screen === 'verify' ? 'verify' : 'google-verify', link.token);
        setLink(null); setPage('login'); await controller.refresh();
      }, 'Email confirmed. Continue in the device where you started, or sign in here.')} />
      <ActionButton label="Back to sign in" secondary disabled={busy} onPress={() => navigate('login')} />
    </>}
    {shown === 'pending' && <>
      <Text style={styles.text}>We sent a verification link to {state.user?.email}. Verify there, then return here. Your destination is saved.</Text>
      <ActionButton label="I've verified my email" disabled={locked} onPress={() => void run(() => controller.refresh())} />
      <ActionButton label="Resend verification email" secondary disabled={locked} onPress={() => void run(() => controller.resend(), 'A new verification email has been requested.')} />
      <ActionButton label="Sign out" secondary disabled={busy} onPress={() => void run(() => controller.logout())} />
    </>}
    {shown === 'google-pending' && <>
      <Text style={styles.text}>{google?.status === 'awaiting_proof' ? 'Google sign-in was interrupted. Start again to choose an account.' : `Confirm the email sent to ${google?.email ?? 'your Google email'}. Return here when it is verified.`}</Text>
      {google?.status === 'ready' ? <ActionButton label="Continue with verified email" disabled={locked} onPress={() => void run(() => controller.exchangeGoogle())} /> : google?.status !== 'awaiting_proof' && <>
        <ActionButton label="I've verified my email" disabled={locked} onPress={() => void run(() => controller.refresh())} />
        <ActionButton label="Resend verification email" secondary disabled={locked} onPress={() => void run(() => controller.resendGoogle(), 'A new verification email has been requested.')} />
      </>}
      <ActionButton label="Cancel Google sign-in" secondary disabled={busy} onPress={() => void run(() => controller.cancelGoogle())} />
    </>}
    {shown === 'google-onboarding' && <>
      <Text style={styles.text}>Google email: {google?.email}. Add a username and password to finish signing up.</Text>
      <AccountForm key={google?.username ?? 'new-google'} kind="google" username={google?.username} locked={locked} onSubmit={(v) => run(() => controller.completeGoogle(v.username, v.password))} />
      <ActionButton label="Cancel Google sign-in" secondary disabled={busy} onPress={() => void run(() => controller.cancelGoogle())} />
    </>}
    {shown === 'account' && <>
      <ActionButton label="My routes" secondary disabled={busy} onPress={() => void run(async () => { await controller.setDestination('/my-routes'); setSettings(false); })} />
      <View style={styles.card}><Text style={styles.label}>Username</Text><Text style={styles.text}>{state.user?.username}</Text><Text style={styles.label}>Email</Text><Text style={styles.text}>{state.user?.email}</Text><Text style={styles.text}>Your username and email cannot be changed.</Text></View>
      <ActionButton label="Change password" disabled={busy} onPress={() => navigate('change')} />
      <ActionButton label="Sign out" secondary disabled={busy} onPress={() => void run(() => controller.logout())} />
      <ActionButton label="Sign out on all devices" secondary disabled={busy} onPress={() => void run(() => controller.logout(true))} />
    </>}
    {shown === 'change' && <><Text style={styles.text}>Changing your password signs out your other devices.</Text>
      <AccountForm kind="change" locked={locked} onSubmit={(v) => run(async () => { await controller.changePassword(v.currentPassword, v.password); navigate('account'); }, 'Password changed. Your other devices have been signed out.')} />
      <ActionButton label="Back to account" secondary disabled={busy} onPress={() => navigate('account')} /></>}
  </Screen>;
}

type Values = { identifier: string; email: string; username: string; password: string; currentPassword: string; confirm: string };
function AccountForm({ kind, locked, username, onSubmit }: { kind: 'login' | 'signup' | 'forgot' | 'reset' | 'change' | 'google'; locked: boolean; username?: string; onSubmit(values: Values): Promise<unknown> }) {
  const [values, setValues] = useState<Values>({ identifier: '', email: '', username: username ?? '', password: '', currentPassword: '', confirm: '' });
  const [errors, setErrors] = useState<Partial<Values>>({});
  const [visible, setVisible] = useState(false);
  const refs = useRef<Partial<Record<keyof Values, TextInput | null>>>({});
  const hasNewPassword = ['signup', 'reset', 'change', 'google'].includes(kind);
  const hasUsername = ['signup', 'google'].includes(kind);
  const fields: (keyof Values)[] = kind === 'forgot' ? ['identifier'] : kind === 'login' ? ['identifier', 'password']
    : kind === 'signup' ? ['email', 'username', 'password', 'confirm'] : kind === 'google' ? ['username', 'password', 'confirm']
    : kind === 'change' ? ['currentPassword', 'password', 'confirm'] : ['password', 'confirm'];
  const labels = { identifier: 'Email or username', email: 'Email', username: 'Username', password: hasNewPassword ? 'New password' : 'Password', currentPassword: 'Current password', confirm: 'Confirm password' };
  async function submit() {
    if (locked) return;
    const next: Partial<Values> = {};
    for (const name of fields) if (!values[name]) next[name] = `${labels[name]} is required.`;
    if (hasUsername) { const error = usernameError(values.username); if (error) next.username = error; }
    if (hasNewPassword) { const error = passwordError(values.password); if (error) next.password = error; if (values.password !== values.confirm) next.confirm = 'Passwords do not match.'; }
    setErrors(next);
    const invalid = fields.find((name) => next[name]);
    if (invalid) { refs.current[invalid]?.focus(); return; }
    // The parent never receives confirmation as an API field. Clear secrets after every attempt.
    try {
      const reason = await onSubmit(values);
      if (reason instanceof AccountError) {
        const field: keyof Values | null = reason.code === 'username_taken' ? 'username' : reason.code === 'email_taken' ? 'email'
          : reason.code === 'invalid_credentials' ? (kind === 'change' ? 'currentPassword' : 'password') : null;
        if (field) { setErrors({ [field]: errorMessage(reason) }); refs.current[field]?.focus(); }
      }
    } finally { setValues((v) => ({ ...v, password: '', confirm: '', currentPassword: '' })); }
  }
  return <View style={{ gap: 16 }}>
    {fields.map((name, index) => {
      const password = ['password', 'currentPassword', 'confirm'].includes(name);
      return <View key={name} style={{ gap: 6 }}><Field label={labels[name]} ref={(input) => { refs.current[name] = input; }}
        value={values[name]} onChangeText={(value) => { setValues((v) => ({ ...v, [name]: value })); setErrors((e) => ({ ...e, [name]: undefined })); }}
        editable={!locked && !(name === 'username' && Boolean(username))} autoCapitalize="none" autoCorrect={false}
        secureTextEntry={password && !visible} keyboardType={name === 'email' ? 'email-address' : 'default'}
        autoComplete={password ? (name === 'currentPassword' || kind === 'login' ? 'current-password' : 'new-password') : name === 'email' ? 'email' : 'username'}
        textContentType={password ? (name === 'currentPassword' || kind === 'login' ? 'password' : 'newPassword') : name === 'email' ? 'emailAddress' : 'username'}
        importantForAutofill="yes" accessibilityHint={errors[name] ?? (name === 'username' ? '3 to 24 letters, numbers, hyphens or underscores' : undefined)}
        returnKeyType={index === fields.length - 1 ? 'done' : 'next'} onSubmitEditing={() => { if (index === fields.length - 1) void submit(); else refs.current[fields[index + 1]]?.focus(); }} />
        <ErrorMessage message={errors[name] ?? null} /></View>;
    })}
    {hasNewPassword && <Text style={styles.text}>Use 8–128 characters, including a number and a punctuation or symbol character.</Text>}
    {kind !== 'forgot' && <ActionButton label={visible ? 'Hide passwords' : 'Show passwords'} secondary disabled={locked} onPress={() => setVisible(!visible)} />}
    <ActionButton label={locked ? 'Please wait' : ({ login: 'Sign in', signup: 'Create account', forgot: 'Send reset link', reset: 'Reset password', change: 'Change password', google: 'Create account' })[kind]} disabled={locked} onPress={() => void submit()} />
  </View>;
}
function GoogleButton({ locked, onPress }: { locked: boolean; onPress(): void }) {
  return <><Text style={[styles.text, { textAlign: 'center' }]}>or</Text><Pressable accessibilityRole="button" accessibilityLabel="Continue with Google" accessibilityState={{ disabled: locked }} disabled={locked} onPress={onPress} style={[styles.button, { backgroundColor: '#fff', flexDirection: 'row', gap: 12 }, locked && styles.disabled]}><Image source={require('../../assets/google-g.png')} style={{ width: 20, height: 20 }} accessibilityIgnoresInvertColors /><Text style={{ color: '#1f1f1f', fontSize: 16, fontWeight: '600' }}>Continue with Google</Text></Pressable></>;
}
