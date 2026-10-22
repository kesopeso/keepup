export class AccountError extends Error {
  code: string; status: number; retryAfter: number;
  constructor(code: string, status = 0, retryAfter = 0) { super(code); this.code = code; this.status = status; this.retryAfter = retryAfter; }
}
export type AuthTransport = <T>(path: string, body?: object, token?: string, signal?: AbortSignal, method?: string) => Promise<T>;

// Native requests never include cookie authority, Origin or browser CSRF headers.
export function createAuthTransport(baseUrl: string, fetcher: typeof fetch = fetch): AuthTransport {
  return async <T>(path: string, body?: object, token?: string, signal?: AbortSignal, method?: string): Promise<T> => {
    const controller = new AbortController();
    const abort = () => controller.abort();
    if (signal?.aborted) abort();
    signal?.addEventListener('abort', abort, { once: true });
    const timeout = setTimeout(abort, 15000);
    let rejectCancelled: () => void = () => {};
    const cancelled = new Promise<never>((_resolve, reject) => {
      rejectCancelled = () => reject(new AccountError('request_cancelled'));
    });
    controller.signal.addEventListener('abort', rejectCancelled, { once: true });
    if (controller.signal.aborted) rejectCancelled();
    try {
      // Some native fetch/body implementations do not settle after abort. Never let one
      // cancelled screen hold the credential queue and block subsequent navigation.
      const response = await Promise.race([fetcher(`${baseUrl}${path}`, {
        method: method ?? (body === undefined ? 'GET' : 'POST'), credentials: 'omit', cache: 'no-store',
        headers: { 'X-KeepUp-Client': 'native', 'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        ...(body === undefined ? {} : { body: JSON.stringify(body) }), signal: controller.signal,
      }), cancelled]);
      if (response.status === 204) return undefined as T;
      let result: unknown;
      try { result = await Promise.race([response.json(), cancelled]); } catch { throw new AccountError(controller.signal.aborted ? 'request_cancelled' : 'invalid_response', response.status); }
      if (!response.ok) {
        const code = (result as { error?: unknown })?.error;
        const retry = Number(response.headers.get('Retry-After'));
        throw new AccountError(typeof code === 'string' ? code : 'server_error', response.status,
          Number.isInteger(retry) && retry > 0 ? Math.min(retry, 86400) : 0);
      }
      return result as T;
    } catch (error) {
      if (error instanceof AccountError) throw error;
      throw new AccountError(controller.signal.aborted ? 'request_cancelled' : 'offline');
    } finally {
      clearTimeout(timeout);
      signal?.removeEventListener('abort', abort);
      controller.signal.removeEventListener('abort', rejectCancelled);
    }
  };
}

export function errorMessage(error: unknown): string {
  const code = error instanceof AccountError ? error.code : 'storage_error';
  return ({ invalid_credentials: 'Email, username or password is incorrect.', email_taken: 'This email is already registered. Try signing in.',
    username_taken: 'This username is taken. Choose another.', invalid_input: 'Check the fields and try again.',
    invalid_token: 'This link is invalid or has already been used. Request a new link.', token_expired: 'This link has expired. Request a new link.',
    session_expired: 'Your session has expired. Please sign in again.', rate_limited: 'Too many attempts. Please wait before trying again.',
    offline: 'Cannot reach KeepUp. Check your connection and try again.', request_cancelled: 'The request timed out or was cancelled. Try again.',
    google_unavailable: 'Google sign-in is unavailable. Use your email and password.', invalid_google_credential: 'Google could not verify this sign-in. Please try again.',
    invalid_flow: 'Google sign-in has ended. Start again.', flow_expired: 'Google sign-in has expired. Start again.',
    identity_conflict: 'This Google identity is linked to another account. Use your email and password.',
    storage_error: 'Could not save account access securely. Retry saving before continuing.',
    cancelled: 'Google sign-in was cancelled. You can try again.',
  } as Record<string, string>)[code] ?? 'Could not complete the request. Please try again.';
}
