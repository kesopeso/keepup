import type {
  RouteMapCallbacks,
  RouteMapRenderer,
  RouteMapState,
} from "./route-map-types";

export class PlaceholderRouteMapRenderer implements RouteMapRenderer {
  private state: RouteMapState = {
    members: [],
    viewportMode: "fit_route",
  };

  constructor(
    private readonly container: HTMLElement,
    private readonly callbacks: RouteMapCallbacks = {},
  ) {}

  render(state: RouteMapState): void {
    this.state = state;
    this.container.dataset.viewportMode = state.viewportMode;
    this.container.dataset.pointCount = String(countPoints(state));
    this.container.dataset.memberCount = String(state.members.length);
  }

  fitToRoute(): void {
    this.state = {
      ...this.state,
      focusedMemberId: undefined,
      viewportMode: "fit_route",
    };
    this.callbacks.onViewportChanged?.("fit_route");
    this.render(this.state);
  }

  fitToMember(memberId: string): void {
    this.state = {
      ...this.state,
      focusedMemberId: memberId,
      viewportMode: "manual",
    };
    this.callbacks.onMemberMarkerClick?.(memberId);
    this.callbacks.onViewportChanged?.("manual");
    this.render(this.state);
  }

  setAutoFollow(enabled: boolean): void {
    this.state = {
      ...this.state,
      viewportMode: enabled ? "fit_route" : "manual",
    };
    this.callbacks.onViewportChanged?.(this.state.viewportMode);
    this.render(this.state);
  }

  destroy(): void {
    delete this.container.dataset.viewportMode;
    delete this.container.dataset.pointCount;
    delete this.container.dataset.memberCount;
  }
}

function countPoints(state: RouteMapState): number {
  return state.members.reduce(
    (total, member) =>
      total +
      member.paths.reduce(
        (memberTotal, path) => memberTotal + path.points.length,
        0,
      ),
    0,
  );
}
