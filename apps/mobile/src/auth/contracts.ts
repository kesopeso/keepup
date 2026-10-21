export type AccountUser = { id: string; email: string; username: string; verified: boolean };
export type AccountSession = { user: AccountUser; expiresAt: string; sessionToken: string };
export type GoogleContext = { status: 'awaiting_proof' | 'onboarding' | 'email_verification_required' | 'ready'; email?: string; username?: string; expiresAt: string };
export type GoogleFlow = { flowToken: string; expiresAt: string };
export type Destination = '/my-routes' | '/account' | '/routes/new' | `/routes/${string}`;
export type LinkIntent = { screen: 'verify' | 'reset' | 'google-verify'; token: string } | { screen: 'destination'; destination: Destination };

export function destination(value: unknown): Destination | null {
  if (typeof value !== 'string') return null;
  if (['/my-routes', '/account', '/routes/new'].includes(value)) return value as Destination;
  return /^\/routes\/[A-Z0-9]{6}$/.test(value) ? value as Destination : null;
}

export function authOrigin(value: string | undefined): string | null {
  if (!value) return null;
  const url = new URL(value);
  if (url.protocol !== 'https:' || url.port || url.username || url.password || url.search || url.hash || url.pathname !== '/') {
    throw new Error('EXPO_PUBLIC_AUTH_LINK_ORIGIN must be an HTTPS origin.');
  }
  return url.origin;
}

// Only the configured, verified app-link host is accepted. Tokens never enter storage.
export function parseAuthLink(raw: string, approvedOrigin: string | null): LinkIntent | null {
  try {
    const url = new URL(raw);
    if (!approvedOrigin || url.origin !== approvedOrigin || url.username || url.password || url.search) return null;
    const screen = ({ '/auth/verify': 'verify', '/auth/reset-password': 'reset', '/auth/google/verify': 'google-verify' } as const)[url.pathname as '/auth/verify'];
    if (screen) {
      const token = new URLSearchParams(url.hash.slice(1)).get('token');
      return token && /^[A-Za-z0-9_-]{43}$/.test(token) ? { screen, token } : null;
    }
    if (url.hash) return null;
    const target = destination(url.pathname);
    return target ? { screen: 'destination', destination: target } : null;
  } catch { return null; }
}

export function passwordError(password: string): string | null {
  const count = [...password].length;
  return count < 8 || count > 128 || !/[0-9]/.test(password) || !/[\p{P}\p{S}]/u.test(password)
    ? 'Use 8–128 characters, including a number and a punctuation or symbol character.' : null;
}
export function usernameError(username: string): string | null {
  return /^[A-Za-z0-9_-]{3,24}$/.test(username) ? null : 'Use 3–24 letters, numbers, hyphens or underscores.';
}

export function isSession(value: unknown): value is AccountSession {
  const s = value as AccountSession | null;
  return Boolean(s && typeof s.sessionToken === 'string' && /^[A-Za-z0-9_-]{43}$/.test(s.sessionToken)
    && typeof s.expiresAt === 'string' && Number.isFinite(Date.parse(s.expiresAt))
    && s.user && typeof s.user.id === 'string' && typeof s.user.email === 'string'
    && typeof s.user.username === 'string' && typeof s.user.verified === 'boolean');
}
