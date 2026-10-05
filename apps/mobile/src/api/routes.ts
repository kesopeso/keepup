import { isTransportMode } from '../domain/routes.ts';
import type { JoinRequest, JoinResponse, RouteAccess, RouteSnapshot } from '../domain/routes.ts';

export const ROUTE_REQUEST_TIMEOUT_MS = 10_000;

export class RouteApiError extends Error {
  status: number;
  code: string;
  constructor(message: string, status = 0, code = 'network') {
    super(message);
    this.name = 'RouteApiError';
    this.status = status;
    this.code = code;
  }
}

function errorMessage(status: number, code: string): string {
  if (code === 'invalid_password') return 'The password is not correct. Please try again.';
  if (code === 'alias_taken') return 'That name is already used on this route. Choose another name.';
  if (code === 'route_closed') return 'This route is closed and is not accepting new members.';
  if (status === 404) return 'Route not found. Check the code or link.';
  if (status === 401 || status === 403) return 'Your saved access is no longer valid. Join the route again.';
  if (status === 400) return 'Check your details and try again.';
  if (status >= 500) return 'KeepUp is temporarily unavailable. Please try again shortly.';
  return 'Could not load the route. Please try again.';
}

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
function validRoute(value: unknown): boolean {
  return record(value) && typeof value.code === 'string' && /^[A-Z0-9]{6}$/.test(value.code) &&
    typeof value.name === 'string' && typeof value.description === 'string' &&
    ['active', 'closed'].includes(String(value.status)) &&
    ['everyone_can_share', 'joiners_can_view_only'].includes(String(value.sharingPolicy));
}
function validSnapshot(value: unknown): boolean {
  return record(value) && validRoute(value.route) && record(value.viewer) &&
    typeof value.viewer.memberId === 'string' && Array.isArray(value.members) &&
    value.members.every((member: unknown) => record(member) && typeof member.id === 'string' &&
      typeof member.displayName === 'string' && isTransportMode(member.transportMode) &&
      ['owner', 'member'].includes(String(member.role)) &&
      ['tracking', 'spectating', 'stale', 'offline', 'left'].includes(String(member.status)) &&
      typeof member.color === 'string' && /^#[a-f0-9]{6}$/i.test(member.color) && Array.isArray(member.paths) &&
      member.paths.every((path: unknown) => record(path) && Array.isArray(path.points) &&
        path.points.every((point: unknown) => record(point) && typeof point.latitude === 'number' &&
          Number.isFinite(point.latitude) && Math.abs(point.latitude) <= 90 && typeof point.longitude === 'number' &&
          Number.isFinite(point.longitude) && Math.abs(point.longitude) <= 180 && typeof point.recordedAt === 'string' &&
          Number.isFinite(Date.parse(point.recordedAt)))));
}

export function createRoutesApi(baseUrl: string) {
  async function request<T>(path: string, signal: AbortSignal, validate: (body: unknown) => boolean, options: RequestInit = {}): Promise<T> {
    const controller = new AbortController();
    const abort = () => controller.abort();
    signal.addEventListener('abort', abort);
    if (signal.aborted) abort();
    let timedOut = false;
    const timeout = setTimeout(() => { timedOut = true; controller.abort(); }, ROUTE_REQUEST_TIMEOUT_MS);
    try {
      const response = await fetch(`${baseUrl}${path}`, {
        ...options, signal: controller.signal, cache: 'no-store',
        headers: { Accept: 'application/json', ...options.headers },
      });
      let body: unknown;
      try {
        body = await response.json();
      } catch (error) {
        if (controller.signal.aborted) throw error;
        if (!response.ok) throw new RouteApiError(errorMessage(response.status, ''), response.status, 'server');
        throw new RouteApiError('KeepUp returned an unexpected response. Please try again.', response.status, 'invalid_response');
      }
      if (!response.ok) {
        const code = record(body) && typeof body.error === 'string' ? body.error : 'server';
        throw new RouteApiError(errorMessage(response.status, code), response.status, code);
      }
      if (!validate(body)) throw new RouteApiError('KeepUp returned an unexpected response. Please try again.', response.status, 'invalid_response');
      return body as T;
    } catch (error) {
      if (signal.aborted) throw error;
      if (error instanceof RouteApiError) throw error;
      throw new RouteApiError(
        timedOut ? 'KeepUp took too long to respond. Please try again.' : 'Could not reach KeepUp. Check your connection and try again.',
        0, timedOut ? 'timeout' : 'network',
      );
    } finally {
      clearTimeout(timeout);
      signal.removeEventListener('abort', abort);
    }
  }

  return {
    getAccess: (code: string, signal: AbortSignal) => request<RouteAccess>(`/routes/${encodeURIComponent(code)}/access`, signal,
      (body) => validRoute(body) && record(body) && body.code === code && typeof body.requiresPassword === 'boolean'),
    join: (code: string, input: JoinRequest, signal: AbortSignal) => request<JoinResponse>(`/routes/${encodeURIComponent(code)}/members`, signal,
      (body) => record(body) && validRoute(body.route) && record(body.route) && body.route.code === code &&
        record(body.member) && typeof body.member.id === 'string' && typeof body.memberToken === 'string' && body.memberToken.length > 0,
      { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) }),
    getSnapshot: (code: string, token: string, signal: AbortSignal) => request<RouteSnapshot>(`/routes/${encodeURIComponent(code)}`, signal,
      (body) => validSnapshot(body) && record(body) && record(body.route) && body.route.code === code,
      { headers: { Authorization: `Bearer ${token}` } }),
  };
}

export type RoutesApi = ReturnType<typeof createRoutesApi>;
export function invalidMembership(error: unknown): boolean {
  return error instanceof RouteApiError && [401, 403, 404].includes(error.status) && error.code !== 'invalid_password';
}
