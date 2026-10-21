export type RouteListItem = { id: string; code: string; name: string; status: "active" | "closed"; relationship: "owner" | "joined" | "left"; hasPassword: boolean; createdAt: string; closedAt: string | null; action: "open" | "view_archive" | "rejoin" };
export type RouteListResult = { items: RouteListItem[]; counts: { active: number; archived: number }; nextCursor: string | null };
export type RouteFilters = { view: "active" | "archived"; owned: boolean; joined: boolean; left: boolean };
export type SavedRouteList = { userId: string; filters: RouteFilters; result: RouteListResult | null; scroll: number };
export const defaultRouteFilters: RouteFilters = { view: "active", owned: true, joined: true, left: false };
const key = "keepup.my-routes.v1";
const returnKey = `${key}.return`;
export function markRouteListReturn() { try { sessionStorage.setItem(returnKey, "1"); } catch { /* Retention is optional. */ } }
export function consumeRouteListReturn() { try { const returning = sessionStorage.getItem(returnKey) === "1"; sessionStorage.removeItem(returnKey); return returning; } catch { return false; } }
export function isRouteListReturn() { try { return sessionStorage.getItem(returnKey) === "1"; } catch { return false; } }
export function routeListQuery(filters: RouteFilters, cursor?: string | null): string {
  return new URLSearchParams({ view: filters.view, owned: String(filters.owned), joined: String(filters.joined), left: String(filters.left), limit: "20", ...(cursor ? { cursor } : {}) }).toString();
}
export function appendRoutePage(previous: RouteListResult, page: RouteListResult): RouteListResult {
  return { ...page, items: [...new Map([...previous.items, ...page.items].map(item => [item.id, item])).values()] };
}
export function clearRouteListState() { try { sessionStorage.removeItem(key); sessionStorage.removeItem(returnKey); } catch { /* Storage can be unavailable. */ } }
export function saveRouteListState(value: SavedRouteList) { try { sessionStorage.setItem(key, JSON.stringify(value)); } catch { /* List still works without retention. */ } }
export function readRouteListState(userId: string): SavedRouteList | null {
  try {
    const value = JSON.parse(sessionStorage.getItem(key) ?? "null");
    if (value?.userId !== userId || !["active", "archived"].includes(value.filters?.view) || ["owned", "joined", "left"].some(key => typeof value.filters[key] !== "boolean")) { clearRouteListState(); return null; }
    if (value.result && (!Array.isArray(value.result.items) || !value.result.counts)) return null;
    return value;
  } catch { return null; }
}
