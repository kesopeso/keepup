export const transportModes = ['walking', 'bicycle', 'car', 'bus', 'train', 'boat', 'airplane'] as const;
export type TransportMode = (typeof transportModes)[number];
export const transportLabels: Record<TransportMode, string> = {
  walking: 'Walking', bicycle: 'Bicycle', car: 'Car', bus: 'Bus', train: 'Train', boat: 'Boat', airplane: 'Airplane',
};
export type MemberStatus = 'tracking' | 'spectating' | 'stale' | 'offline' | 'left';
export type RouteStatus = 'active' | 'closed';
export type SharingPolicy = 'everyone_can_share' | 'joiners_can_view_only';
export type Profile = { clientId: string; displayName: string; transportMode: TransportMode };
export type Membership = { code: string; memberId: string; memberToken: string };
export type RouteAccess = {
  code: string; name: string; description: string; status: RouteStatus;
  requiresPassword: boolean; sharingPolicy: SharingPolicy;
};
export type RouteSummary = Omit<RouteAccess, 'requiresPassword'> & {
  id: string; hasPassword: boolean; maxTrackingMembers: number; createdAt: string; closedAt: string | null;
};
export type RoutePoint = {
  latitude: number; longitude: number; recordedAt: string;
  seq?: number; accuracyM?: number; clientRecordedAt?: string;
};
export type PathSegment = { id?: string; startedAt?: string; endedAt?: string; points: RoutePoint[] };
export type SnapshotMember = {
  id: string; displayName: string; transportMode: TransportMode; role: 'owner' | 'member';
  status: MemberStatus; color: string; joinedAt: string; leftAt: string | null; paths: PathSegment[];
};
export type RouteSnapshot = {
  route: RouteSummary;
  members: SnapshotMember[];
  viewer: {
    memberId: string; role: 'owner' | 'member'; status: MemberStatus;
    canStartSharing: boolean; canStopSharing: boolean; canLeaveRoute: boolean;
    canCloseRoute: boolean; canDeleteRoute: boolean; canEditRoute: boolean;
  };
};
export type JoinRequest = Profile & { password: string };
export type JoinResponse = { route: RouteSummary; member: { id: string }; memberToken: string };

export function isTransportMode(value: unknown): value is TransportMode {
  return typeof value === 'string' && transportModes.includes(value as TransportMode);
}

export function parseRouteCode(input: string): string {
  const value = input.trim();
  if (/^[a-z0-9]{6}$/i.test(value)) return value.toUpperCase();
  try {
    const url = new URL(value);
    if (!['http:', 'https:'].includes(url.protocol)) throw new Error();
    const match = /^\/routes\/([a-z0-9]{6})\/?$/i.exec(decodeURIComponent(url.pathname));
    if (match) return match[1].toUpperCase();
  } catch {
    // Neither a route code nor a supported share link.
  }
  throw new Error('Enter a six-character route code or a KeepUp route link.');
}
