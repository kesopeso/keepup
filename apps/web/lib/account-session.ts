import { authRequest, AuthError, type AccountSession } from "./auth-api.ts";

const eventKey = "keepup.account.changed.v1";
let session: AccountSession | null = null;
let revision = 0;
const listeners = new Set<() => void>();
export function currentSession() { return session; }
export function subscribeSession(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; }
function publish() { for (const listener of listeners) listener(); }
export function setAccountSession(value: AccountSession | null, broadcast = true) {
  revision++;
  session = value;
  publish();
  if (broadcast) {
    try { localStorage.setItem(eventKey, `${Date.now()}-${Math.random()}`); } catch { /* Polling/focus still rechecks authority. */ }
  }
}
export async function refreshAccountSession(): Promise<AccountSession | null> {
  const version = revision;
  try {
    const next = await authRequest<AccountSession>("me");
    if (version === revision) { session = next; publish(); }
  } catch (error) {
    if (version === revision && error instanceof AuthError && ["session_expired", "auth_unavailable"].includes(error.code)) {
      session = null; publish();
    } else throw error;
  }
  return session;
}
// Screens opt into checks; legacy route/landing pages do not call account endpoints.
export function watchAccountSession(onError: (error: unknown) => void) {
  let checking = false;
  let stopped = false;
  const check = async () => {
    if (checking || stopped) return;
    checking = true;
    try { await refreshAccountSession(); } catch (error) { if (!stopped) onError(error); }
    finally { checking = false; }
  };
  const storage = (event: StorageEvent) => { if (event.key === eventKey) void check(); };
  const timer = window.setInterval(() => { if (document.visibilityState !== "hidden") void check(); }, 5000);
  window.addEventListener("focus", check);
  window.addEventListener("online", check);
  window.addEventListener("storage", storage);
  document.addEventListener("visibilitychange", check);
  return () => {
    stopped = true;
    clearInterval(timer);
    window.removeEventListener("focus", check);
    window.removeEventListener("online", check);
    window.removeEventListener("storage", storage);
    document.removeEventListener("visibilitychange", check);
  };
}
