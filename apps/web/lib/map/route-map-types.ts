export type RouteMapViewportMode = "fit_route" | "manual";

export type RouteMapPoint = {
  latitude: number;
  longitude: number;
  recordedAt: string;
};

export type RouteMapPath = {
  id?: string;
  points: RouteMapPoint[];
};

export type RouteMapMember = {
  id: string;
  displayName: string;
  color: string;
  status: string;
  transportMode: string;
  paths: RouteMapPath[];
  latestPoint?: RouteMapPoint;
};

export type RouteMapState = {
  members: RouteMapMember[];
  viewportMode: RouteMapViewportMode;
  focusedMemberId?: string;
};

export type RouteMapCallbacks = {
  onViewportChanged?: (mode: RouteMapViewportMode) => void;
  onMemberMarkerClick?: (memberId: string) => void;
  onMapInteraction?: () => void;
};

export type RouteMapRenderer = {
  render(state: RouteMapState): void;
  fitToRoute(): void;
  fitToMember(memberId: string): void;
  setAutoFollow(enabled: boolean): void;
  destroy(): void;
};

export type RouteMapRendererFactory = (
  container: HTMLElement,
  callbacks?: RouteMapCallbacks,
) => RouteMapRenderer;
