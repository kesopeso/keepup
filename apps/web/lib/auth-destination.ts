const key = "keepup.auth.destination.v1";
const ttl = 24 * 60 * 60 * 1000;
export function safeDestination(value: string | null | undefined): string | null {
  if (!value) return null;
  if (["/my-routes", "/account", "/routes/new"].includes(value)) return value;
  return /^\/routes\/[A-Za-z0-9]{6}$/.test(value) ? `/routes/${value.slice(8).toUpperCase()}` : null;
}
export function rememberDestination(value: string | null): void {
  const path = safeDestination(value);
  try {
    // Explicit invalid input must replace a stale, previously saved destination.
    if (path) localStorage.setItem(key, JSON.stringify({ path, expires: Date.now() + ttl }));
    else localStorage.removeItem(key);
  } catch { /* URL next parameter still works when storage is blocked. */ }
}
export function savedDestination(): string {
  try {
    const value = JSON.parse(localStorage.getItem(key) ?? "null");
    if (value?.expires > Date.now()) return safeDestination(value.path) ?? "/my-routes";
    localStorage.removeItem(key);
  } catch { /* No persistent destination. */ }
  return "/my-routes";
}
export async function checkedDestination(path: string): Promise<{ path: string; unavailable: boolean }> {
  path = safeDestination(path) ?? "/my-routes";
  if (/^\/routes\/[A-Z0-9]{6}$/.test(path)) {
    // Access metadata is available before membership, so invitations can resume joining.
    const response = await fetch(`/api${path}/access`, { credentials: "same-origin", cache: "no-store" });
    if (response.status === 401) throw new Error("session_expired");
    if (response.status === 403 || response.status === 404) return { path: "/my-routes", unavailable: true };
    if (!response.ok) throw new Error("destination_unavailable");
  }
  return { path, unavailable: false };
}
