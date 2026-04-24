import type { TransportMode } from "./identity-storage";

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

const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

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
): Promise<CreateRouteResponse> {
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

function routeErrorMessage(status: number, code?: string): string {
  if (code === "invalid_input" || status === 400) {
    return "Check the route details and try again.";
  }

  if (status >= 500) {
    return "The route service is unavailable. Try again shortly.";
  }

  return "Could not create the route.";
}
