import { AccountError } from './api.ts';
import type { AccountController } from './controller.ts';
import { validSnapshot } from '../api/routes.ts';
import type { RouteAccess, RouteSnapshot, SharingPolicy, TransportMode } from '../domain/routes.ts';
export type RouteFilters = { view: 'active' | 'archived'; owned: boolean; joined: boolean; left: boolean };
export type RouteListItem = { id: string; code: string; name: string; status: 'active' | 'closed'; relationship: 'owner' | 'joined' | 'left'; hasPassword: boolean; createdAt: string; closedAt: string | null; action: 'open' | 'view_archive' | 'rejoin' };
export type RouteList = { items: RouteListItem[]; counts: { active: number; archived: number }; nextCursor: string | null };
export const defaultFilters: RouteFilters = { view: 'active', owned: true, joined: true, left: false };
export type RouteListCache = { filters: RouteFilters; result: RouteList | null; scroll: number; pages: number };
export function freshList(): RouteListCache { return { filters: { ...defaultFilters }, result: null, scroll: 0, pages: 1 }; }
export function listQuery(filters: RouteFilters, cursor?: string | null) {
  return new URLSearchParams({ view: filters.view, owned: String(filters.owned), joined: String(filters.joined), left: String(filters.left), limit: '20', ...(cursor ? { cursor } : {}) }).toString();
}
export function appendPage(previous: RouteList, next: RouteList): RouteList {
  return { ...next, items: [...new Map([...previous.items, ...next.items].map((item) => [item.id, item])).values()] };
}
export function routeError(error: unknown): string {
  if (!(error instanceof AccountError)) return 'KeepUp returned an unexpected response. Try again.';
  return ({ invalid_password: 'The route password is incorrect.', route_not_found: 'Route not found. Check the code or link.',
    unauthorized: 'Join this route to view it.', route_closed: 'This route is closed.', sharing_on_another_device: 'Sharing is active on another device.',
    session_expired: 'Your session ended. Sign in again.', rate_limited: 'Too many requests. Please wait and retry.',
    offline: 'You are offline. Cached routes are read-only. Check your connection and retry.',
    request_cancelled: 'The request timed out or was cancelled. Retry when connected.',
  } as Record<string, string>)[error.code] ?? 'Could not load the route. Please retry.';
}
export function invalidAccountRoute(error: unknown) {
  return error instanceof AccountError && [401, 403, 404].includes(error.status) && error.code !== 'invalid_password';
}
export function createAccountRoutesApi(controller: AccountController) {
  const request = controller.routeRequest;
  return {
    async list(filters: RouteFilters, signal: AbortSignal, cursor?: string | null) {
      const value = await request<RouteList>(`/me/routes?${listQuery(filters, cursor)}`, undefined, signal);
      if (!value || !Array.isArray(value.items) || !Number.isSafeInteger(value.counts?.active) || !Number.isSafeInteger(value.counts?.archived)
        || !(value.nextCursor === null || typeof value.nextCursor === 'string') || !value.items.every((item) => item && typeof item.id === 'string'
          && /^[A-Z0-9]{6}$/.test(item.code) && typeof item.name === 'string' && ['active', 'closed'].includes(item.status)
          && ['owner', 'joined', 'left'].includes(item.relationship) && ['open', 'view_archive', 'rejoin'].includes(item.action)
          && typeof item.hasPassword === 'boolean' && Number.isFinite(Date.parse(item.createdAt))
          && (item.closedAt === null || Number.isFinite(Date.parse(item.closedAt))))) throw new AccountError('invalid_response');
      return value;
    },
    async access(code: string, signal: AbortSignal) {
      const value = await request<RouteAccess>(`/routes/${code}/access`, undefined, signal);
      if (value?.code !== code || typeof value.name !== 'string' || typeof value.requiresPassword !== 'boolean'
        || !['active', 'closed'].includes(value.status)) throw new AccountError('invalid_response');
      return value;
    },
    async snapshot(code: string, signal: AbortSignal) {
      const result = await request<RouteSnapshot>(`/routes/${code}`, undefined, signal);
      if (!validSnapshot(result) || result.route.code !== code || !['owner', 'member'].includes(result.viewer.role)
        || ['canStartSharing', 'canStopSharing', 'canLeaveRoute', 'canCloseRoute', 'canDeleteRoute', 'canEditRoute']
          .some((key) => typeof result.viewer[key as keyof typeof result.viewer] !== 'boolean')) throw new AccountError('invalid_response');
      return result;
    },
    async create(input: { name: string; description: string; password: string; sharingPolicy: SharingPolicy; transportMode: TransportMode }, signal: AbortSignal) {
      const result = await request<{ route: { code: string } }>('/routes', input, signal);
      if (!/^[A-Z0-9]{6}$/.test(result?.route?.code)) throw new AccountError('invalid_response');
      return result.route.code;
    },
    join: (code: string, input: { password?: string; transportMode?: TransportMode }, signal: AbortSignal) => request(`/routes/${code}/members`, input, signal),
    async ticket(code: string, signal: AbortSignal) {
      const result = await request<{ ticket: string; expiresAt: string }>(`/routes/${code}/socket-ticket`, {}, signal);
      if (!/^[A-Za-z0-9_-]{43}$/.test(result?.ticket) || !Number.isFinite(Date.parse(result.expiresAt)) || Date.parse(result.expiresAt) <= Date.now()) throw new AccountError('invalid_response');
      return result.ticket;
    },
    leave: (code: string) => request(`/routes/${code}/members/me`, {}, undefined, 'DELETE'),
    close: (snapshot: RouteSnapshot) => request(`/routes/${snapshot.route.code}`, { name: snapshot.route.name, description: snapshot.route.description, status: 'closed' }, undefined, 'PATCH'),
    update: (code: string, name: string, description: string) => request(`/routes/${code}`, { name: name.trim(), description: description.trim() }, undefined, 'PATCH'),
    remove: (code: string) => request(`/routes/${code}`, {}, undefined, 'DELETE'),
  };
}
export type AccountRoutesApi = ReturnType<typeof createAccountRoutesApi>;
