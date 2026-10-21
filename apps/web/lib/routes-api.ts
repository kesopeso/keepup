import type { TransportMode } from "./identity-storage";
import { accountRequest, AuthError } from "./auth-api.ts";
import { setAccountSession } from "./account-session.ts";

export const accountAuthority = "account-cookie";
export async function accountRouteRequest<T>(path: string, method = "GET", body?: object): Promise<T> {
  try { return await accountRequest<T>(path, method, body); }
  catch (error) {
    if (error instanceof AuthError && error.code === "session_expired") setAccountSession(null);
    if (error instanceof AuthError) throw new ApiError(error.code === "offline" ? "Cannot reach KeepUp. Check your connection and try again." : routeErrorMessage(error.status, error.code), error.status, error.code);
    throw error;
  }
}

export type SharingPolicy = "everyone_can_share" | "joiners_can_view_only";

export type RouteSummary = {
  id: string;
  code: string;
  name: string;
  description: string;
  hasPassword: boolean;
  sharingPolicy: SharingPolicy;
  status: "active" | "closed";
  maxTrackingMembers: number;
  createdAt: string;
  closedAt: string | null;
};

export type MemberSummary = {
  id: string;
  routeId: string;
  clientId: string;
  displayName: string;
  transportMode: TransportMode;
  isOwner: boolean;
  status: string;
  color: string;
  joinedAt: string;
  leftAt: string | null;
};

export type CreateRouteRequest = {
  clientId: string;
  displayName: string;
  transportMode: TransportMode;
  name: string;
  description: string;
  password: string;
  sharingPolicy: SharingPolicy;
};

export type CreateRouteResponse = {
  route: RouteSummary;
  owner: MemberSummary;
  memberToken: string;
  ownerToken: string;
};

export type RouteAccess = {
  code: string;
  name: string;
  description: string;
  status: "active" | "closed";
  requiresPassword: boolean;
  sharingPolicy: SharingPolicy;
};

export type JoinRouteRequest = {
  clientId: string;
  displayName: string;
  transportMode: TransportMode;
  password: string;
};

export type JoinRouteResponse = {
  route: RouteSummary;
  member: MemberSummary;
  memberToken: string;
};

export type SnapshotMember = {
  id: string;
  displayName: string;
  transportMode: TransportMode;
  role: "owner" | "member";
  status: string;
  color: string;
  joinedAt: string;
  leftAt: string | null;
  paths: PathSegment[];
};

export type PathSegment = {
  id?: string;
  startedAt?: string;
  endedAt?: string;
  points: RoutePoint[];
};

export type RoutePoint = {
  seq?: number;
  latitude: number;
  longitude: number;
  accuracyM?: number;
  clientRecordedAt?: string;
  recordedAt: string;
};

export type ViewerCapabilities = {
  memberId: string;
  role: "owner" | "member";
  status: string;
  canStartSharing: boolean;
  canStopSharing: boolean;
  canLeaveRoute: boolean;
  canCloseRoute: boolean;
  canDeleteRoute: boolean;
  canEditRoute: boolean;
};

export type RouteSnapshot = {
  route: RouteSummary;
  members: SnapshotMember[];
  viewer: ViewerCapabilities;
};

const apiUrl = "/api";

export function routeWebSocketUrl(): string {
  const url = new URL("/ws", window.location.origin);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString();
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function createRoute(
  request: CreateRouteRequest,
  account = false,
): Promise<CreateRouteResponse> {
  if (account) {
    const { clientId: _clientId, displayName: _displayName, ...body } = request;
    return accountRouteRequest("/routes", "POST", body);
  }
  const response = await fetch(`${apiUrl}/routes`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    let errorCode: string | undefined;
    try {
      const payload = (await response.json()) as { error?: string };
      errorCode = payload.error;
    } catch {
      errorCode = undefined;
    }

    throw new ApiError(
      routeErrorMessage(response.status, errorCode),
      response.status,
      errorCode,
    );
  }

  return (await response.json()) as CreateRouteResponse;
}

export async function getRouteAccess(code: string, account = false): Promise<RouteAccess> {
  if (account) return accountRouteRequest(`/routes/${encodeURIComponent(code)}/access`);
  const response = await fetch(`${apiUrl}/routes/${encodeURIComponent(code)}/access`);

  if (!response.ok) {
    let errorCode: string | undefined;
    try {
      const payload = (await response.json()) as { error?: string };
      errorCode = payload.error;
    } catch {
      errorCode = undefined;
    }

    throw new ApiError(
      routeErrorMessage(response.status, errorCode),
      response.status,
      errorCode,
    );
  }

  return (await response.json()) as RouteAccess;
}

export async function joinRoute(
  code: string,
  request: JoinRouteRequest,
  account = false,
): Promise<JoinRouteResponse> {
  if (account) return accountRouteRequest(`/routes/${encodeURIComponent(code)}/members`, "POST", { transportMode: request.transportMode, password: request.password });
  const response = await fetch(`${apiUrl}/routes/${encodeURIComponent(code)}/members`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    let errorCode: string | undefined;
    try {
      const payload = (await response.json()) as { error?: string };
      errorCode = payload.error;
    } catch {
      errorCode = undefined;
    }

    throw new ApiError(
      routeErrorMessage(response.status, errorCode),
      response.status,
      errorCode,
    );
  }

  return (await response.json()) as JoinRouteResponse;
}

export async function getRouteSnapshot(
  code: string,
  memberToken: string,
): Promise<RouteSnapshot> {
  if (memberToken === accountAuthority) return accountRouteRequest(`/routes/${encodeURIComponent(code)}`);
  const response = await fetch(`${apiUrl}/routes/${encodeURIComponent(code)}`, {
    headers: {
      Authorization: `Bearer ${memberToken}`,
    },
  });

  if (!response.ok) {
    let errorCode: string | undefined;
    try {
      const payload = (await response.json()) as { error?: string };
      errorCode = payload.error;
    } catch {
      errorCode = undefined;
    }

    throw new ApiError(
      routeErrorMessage(response.status, errorCode),
      response.status,
      errorCode,
    );
  }

  return (await response.json()) as RouteSnapshot;
}

export async function closeRoute(
  code: string,
  ownerToken: string,
): Promise<RouteSummary> {
  if (ownerToken === accountAuthority) return accountRouteRequest(`/routes/${encodeURIComponent(code)}`, "PATCH", { status: "closed" });
  const response = await fetch(`${apiUrl}/routes/${encodeURIComponent(code)}`, {
    method: "PATCH",
    headers: {
      Authorization: `Bearer ${ownerToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ status: "closed" }),
  });

  if (!response.ok) {
    throw await routeApiError(response);
  }

  return (await response.json()) as RouteSummary;
}

export async function deleteRoute(code: string, ownerToken: string): Promise<void> {
  if (ownerToken === accountAuthority) return accountRouteRequest(`/routes/${encodeURIComponent(code)}`, "DELETE", {});
  const response = await fetch(`${apiUrl}/routes/${encodeURIComponent(code)}`, {
    method: "DELETE",
    headers: {
      Authorization: `Bearer ${ownerToken}`,
    },
  });

  if (!response.ok) {
    throw await routeApiError(response);
  }
}

async function routeApiError(response: Response): Promise<ApiError> {
  let errorCode: string | undefined;
  try {
    const payload = (await response.json()) as { error?: string };
    errorCode = payload.error;
  } catch {
    errorCode = undefined;
  }

  return new ApiError(
    routeErrorMessage(response.status, errorCode),
    response.status,
    errorCode,
  );
}

function routeErrorMessage(status: number, code?: string): string {
  if (code === "sharing_on_another_device") return "You are sharing this route on another device. Stop sharing there before starting here.";
  if (code === "session_expired") return "Your session expired. Sign in again.";
  if (code === "invalid_input" || status === 400) {
    return "Check the details and try again.";
  }

  if (code === "invalid_password") {
    return "The password is not correct.";
  }

  if (code === "unauthorized" || status === 401 || status === 403) {
    return "Route access expired. Join again to continue.";
  }

  if (status === 404) {
    return "Route not found.";
  }

  if (code === "sharing_not_allowed") {
    return "This route only allows the owner to share location.";
  }

  if (code === "tracking_limit_reached") {
    return "All tracking slots are currently in use.";
  }

  if (code === "route_closed") {
    return "This route is closed.";
  }

  if (status === 409) {
    return "That name is already used on this route.";
  }

  if (status >= 500) {
    return "The route service is unavailable. Try again shortly.";
  }

  return "Could not load the route.";
}
