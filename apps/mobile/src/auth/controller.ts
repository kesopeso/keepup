import { AccountError } from './api.ts';
import type { AuthTransport } from './api.ts';
import { isSession } from './contracts.ts';
import type { AccountSession, AccountUser, GoogleContext, GoogleFlow, Destination } from './contracts.ts';
import type { AccountRepository } from './repository.ts';
export type AccountState = { ready: boolean; user: AccountUser | null; google: GoogleContext | null; destination: Destination; saving: boolean };

// One queue owns the credential and its persistence. React sees public identity only.
export function createAccountController(api: AuthTransport, repository: AccountRepository) {
  let session: AccountSession | null = null;
  let flow: GoogleFlow | null = null;
  let pendingSave: AccountSession | null = null;
  let chain: Promise<unknown> = Promise.resolve();
  let generation = 0;
  let state: AccountState = { ready: false, user: null, google: null, destination: '/my-routes', saving: false };
  const subscribers = new Set<() => void>();
  function publish(update: Partial<AccountState>) { state = { ...state, ...update }; subscribers.forEach((fn) => fn()); }
  function serial<T>(work: () => Promise<T>): Promise<T> { const result = chain.then(work); chain = result.catch(() => {}); return result; }
  async function accept(value: unknown) {
    if (!isSession(value)) throw new AccountError('invalid_response');
    pendingSave = value;
    publish({ saving: true });
    await repository.saveSession(value);
    if (flow) await repository.clearFlow();
    flow = null; session = value; pendingSave = null;
    publish({ ready: true, user: value.user, google: null, saving: false });
  }
  async function clearSession() {
    session = null; pendingSave = null;
    publish({ user: null, saving: false });
    await repository.clearSession();
  }
  async function request<T>(path: string, body?: object): Promise<T> {
    try { return await api<T>(path, body, session?.sessionToken); }
    catch (error) {
      if (error instanceof AccountError && error.code === 'session_expired') await clearSession();
      throw error;
    }
  }
  async function googleRequest<T>(path: string, body?: object): Promise<T> {
    if (!flow) throw new AccountError('invalid_flow');
    try { return await api<T>(path, body, flow.flowToken); }
    catch (error) {
      if (error instanceof AccountError && ['invalid_flow', 'flow_expired'].includes(error.code)) {
        flow = null; await repository.clearFlow(); publish({ google: null });
      }
      throw error;
    }
  }
  async function googleResult(value: GoogleContext | ({ status: 'authenticated' } & AccountSession)) {
    if (value.status === 'authenticated') await accept(value);
    else {
      if (!['awaiting_proof', 'onboarding', 'email_verification_required', 'ready'].includes(value.status)
        || !Number.isFinite(Date.parse(value.expiresAt))) throw new AccountError('invalid_response');
      publish({ google: value });
    }
  }
  async function refreshInner() {
    // Do not probe the superseded credential while a rotated session awaits secure storage.
    if (pendingSave) return;
    if (session) {
      const value = await request<{ user: AccountUser; expiresAt: string }>('/auth/me');
      const updated = { ...value, sessionToken: session?.sessionToken };
      if (!isSession(updated)) throw new AccountError('invalid_response');
      // This credential did not rotate. Keep forms mounted while renewing its expiry.
      await repository.saveSession(updated);
      session = updated;
      publish({ user: updated.user });
    } else if (flow) await googleResult(await googleRequest<GoogleContext>('/auth/google/context'));
  }
  return {
    routeRequest: <T>(path: string, body?: object, signal?: AbortSignal, method?: string) => serial(async (): Promise<T> => {
      if (!session?.user.verified || pendingSave) throw new AccountError('session_expired', 401);
      if (signal?.aborted) throw new AccountError('request_cancelled');
      try { return await api<T>(path, body, session.sessionToken, signal, method); }
      catch (error) {
        if (error instanceof AccountError && error.code === 'session_expired') await clearSession();
        throw error;
      }
    }),
    revoke: () => serial(clearSession),
    snapshot: () => state,
    subscribe(fn: () => void) { subscribers.add(fn); return () => { subscribers.delete(fn); }; },
    initialize: () => serial(async () => {
      session = await repository.session(); flow = await repository.flow();
      publish({ user: session?.user ?? null, destination: await repository.destination() ?? '/my-routes' });
      try { await refreshInner(); } finally { publish({ ready: true }); }
    }),
    refresh: () => serial(refreshInner),
    retrySave: () => serial(async () => { if (pendingSave) await accept(pendingSave); }),
    login: (identifier: string, password: string) => serial(async () => accept(await api('/auth/login', { identifier, password }))),
    signup: (email: string, username: string, password: string) => serial(async () => accept(await api('/auth/signup', { email, username, password }))),
    forgot: (identifier: string) => serial(() => api('/auth/password/forgot', { identifier })),
    resend: () => serial(() => request('/auth/verification/resend', {})),
    confirm: (kind: 'verify' | 'google-verify', token: string) => serial(() => api(kind === 'verify' ? '/auth/verification/confirm' : '/auth/google/email/confirm', { token })),
    reset: (token: string, password: string) => serial(async () => { await api('/auth/password/reset', { token, password }); await clearSession(); }),
    changePassword: (currentPassword: string, password: string) => serial(async () => accept(await request('/auth/password/change', { currentPassword, password }))),
    logout: (all = false) => {
      ++generation;
      return serial(async () => {
        if (session) {
          try { await request(all ? '/auth/logout-all' : '/auth/logout', {}); }
          catch (error) { if (!(error instanceof AccountError && error.code === 'session_expired')) throw error; }
        }
        await clearSession(); flow = null; await repository.clearFlow(); await repository.clearDestination();
        publish({ google: null, destination: '/my-routes' });
      });
    },
    setDestination: (path: Destination) => serial(async () => { await repository.saveDestination(path); publish({ destination: path }); }),
    async checkDestination() {
      return serial(async () => {
        const target = state.destination;
        if (!session?.user.verified || !/^\/routes\/[A-Z0-9]{6}$/.test(target)) return target;
        try { await request(`${target}/access`); return target; }
        catch (error) {
          if (error instanceof AccountError && [403, 404].includes(error.status)) {
            await repository.clearDestination(); publish({ destination: '/my-routes' }); return '/my-routes';
          }
          throw error;
        }
      });
    },
    // Native picker runs outside the HTTP queue. Generation fences cancellation/late callbacks.
    async startGoogle(pick: (nonce: string) => Promise<string | null>) {
      const epoch = ++generation;
      const started = await serial(async () => {
        await repository.clearFlow(); flow = null; publish({ google: null });
        const value = await api<GoogleFlow & { nonce: string }>('/auth/google/start', {});
        if (!/^[A-Za-z0-9_-]{43}$/.test(value.flowToken) || !/^[A-Za-z0-9_-]{43}$/.test(value.nonce)
          || Date.parse(value.expiresAt) <= Date.now()) throw new AccountError('invalid_response');
        if (epoch !== generation) return null;
        flow = { flowToken: value.flowToken, expiresAt: value.expiresAt }; await repository.saveFlow(flow);
        return value;
      });
      if (!started || epoch !== generation) return;
      let proof: string | null;
      try { proof = await pick(started.nonce); }
      catch (error) { if (epoch === generation) await this.cancelGoogle(); throw error; }
      if (epoch !== generation) return;
      if (!proof) { await this.cancelGoogle(); throw new AccountError('cancelled'); }
      await serial(async () => { if (epoch === generation) await googleResult(await googleRequest('/auth/google/proof', { idToken: proof })); });
    },
    cancelGoogle() {
      ++generation;
      return serial(async () => { flow = null; await repository.clearFlow(); publish({ google: null }); });
    },
    completeGoogle: (username: string, password: string) => serial(async () => googleResult(await googleRequest('/auth/google/complete', { username, password }))),
    exchangeGoogle: () => serial(async () => googleResult(await googleRequest('/auth/google/exchange', {}))),
    resendGoogle: () => serial(() => googleRequest('/auth/google/email/resend', {})),
  };
}
export type AccountController = ReturnType<typeof createAccountController>;
