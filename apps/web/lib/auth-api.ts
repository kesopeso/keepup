export type AccountUser = { id: string; email: string; username: string; verified: boolean };
export type AccountSession = { user: AccountUser; expiresAt: string };
export type GoogleContext = { status: "awaiting_proof" | "onboarding" | "email_verification_required" | "ready"; email?: string; username?: string; expiresAt: string };
export type GoogleResult = GoogleContext | ({ status: "authenticated" } & AccountSession);

export class AuthError extends Error {
  code: string;
  status: number;
  retryAfter: number;
  constructor(code: string, status = 0, retryAfter = 0) {
    super(code);
    this.code = code;
    this.status = status;
    this.retryAfter = retryAfter;
  }
}

let pending: Promise<unknown> = Promise.resolve();
export function authRequest<T>(path: string, body?: object): Promise<T> {
  const execute = () => {
    // A delayed /me response must not restore the cookie that password change rotated.
    // Web Locks also serialize cookie-changing requests across tabs where supported.
    if (typeof navigator !== "undefined" && navigator.locks) {
      return navigator.locks.request("keepup.account.http", () => sendAuthRequest<T>(path, body));
    }
    return sendAuthRequest<T>(path, body);
  };
  const result = pending.then(execute, execute);
  pending = result.catch(() => undefined);
  return result;
}

async function sendAuthRequest<T>(path: string, body?: object): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api/auth/${path}`, {
      method: body === undefined ? "GET" : "POST",
      credentials: "same-origin",
      cache: "no-store",
      signal: AbortSignal.timeout(15000),
      headers: body === undefined ? {} : { "Content-Type": "application/json", "X-KeepUp-Request": "1" },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    });
  } catch {
    throw new AuthError("offline");
  }
  if (!response.ok) {
    const value = await response.json().catch(() => ({}));
    const retry = Number(response.headers.get("Retry-After"));
    throw new AuthError(value.error ?? (response.status === 404 ? "auth_unavailable" : "internal_error"), response.status, Number.isFinite(retry) ? Math.max(0, retry) : 0);
  }
  return response.status === 204 ? undefined as T : response.json();
}

export const passwordHint = "8–128 characters, including a number and a punctuation or symbol character.";
export function passwordError(password: string): string | undefined {
  const length = Array.from(password).length;
  if (length < 8 || length > 128 || !/[0-9]/.test(password) || !/[\p{P}\p{S}]/u.test(password)) return passwordHint;
}
export function usernameError(username: string): string | undefined {
  if (!/^[A-Za-z0-9_-]{3,24}$/.test(username)) return "Use 3–24 letters, numbers, hyphens (-), or underscores (_).";
}

const messages: Record<string, string> = {
  offline: "Cannot reach KeepUp. Check your connection and try again.",
  invalid_credentials: "The email, username, or password is incorrect.",
  session_expired: "Your session has expired or was logged out. Sign in again.",
  email_taken: "This email is already registered. Sign in or reset your password.",
  username_taken: "This username is already taken. Choose another.",
  invalid_input: "Check the fields and try again.",
  email_verification_required: "Verify your email before continuing.",
  invalid_token: "This link is invalid, already used, or replaced. Request a new email.",
  token_expired: "This link has expired. Request a new email.",
  invalid_flow: "This Google sign-in is no longer available. Start again.",
  flow_expired: "This Google sign-in has expired. Start again.",
  google_unavailable: "Google sign-in is unavailable. Use your email or username, or try again later.",
  invalid_google_credential: "Google sign-in could not be verified. Start again.",
  identity_conflict: "This Google identity is already linked to another account.",
  rate_limited: "Too many attempts. Wait before trying again.",
  auth_unavailable: "Account sign-in is not enabled on this server yet.",
};
export function authMessage(error: unknown): string {
  return error instanceof AuthError ? messages[error.code] ?? "Something went wrong. Try again." : "Something went wrong. Try again.";
}
