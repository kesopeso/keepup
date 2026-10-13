import type {
  GeoJSONSource,
  LngLatBoundsLike,
  Map as MapLibreMap,
  MapLayerMouseEvent,
  Marker,
} from "maplibre-gl";
import { memberLabelOffset, memberLocationStatus, visibleMemberLabels } from "./member-labels";
import { routeMapStyle } from "./tile-provider";
import type {
  RouteMapCallbacks,
  RouteMapMember,
  RouteMapPoint,
  RouteMapRenderer,
  RouteMapState,
  RouteMapViewportMode,
} from "./route-map-types";

type GeoJsonFeatureCollection = GeoJSON.FeatureCollection<GeoJSON.Geometry>;
type MapInteractionEvent = {
  originalEvent?: MouseEvent | TouchEvent | WheelEvent;
};

const pathSourceId = "route-paths";
const markerSourceId = "route-markers";
const pathLayerId = "route-path-lines";
const markerHaloLayerId = "route-marker-halos";
const markerLayerId = "route-member-markers";

export class MapLibreRouteMapRenderer implements RouteMapRenderer {
  private map: MapLibreMap | null = null;
  private mapElement: HTMLDivElement;
  private labels = new Map<string, { marker: Marker; element: HTMLButtonElement }>();
  private createLabelMarker?: (element: HTMLButtonElement) => Marker;
  private mapReady = false;
  private destroyed = false;
  private pendingUserMapInteraction = false;
  private userMapInteractionReset: ReturnType<typeof setTimeout> | null = null;
  private state: RouteMapState = {
    members: [],
    viewportMode: "fit_route",
  };
  private initPromise: Promise<void>;
  private resizeObserver: ResizeObserver;

  constructor(
    private readonly container: HTMLElement,
    private readonly callbacks: RouteMapCallbacks = {},
  ) {
    this.mapElement = document.createElement("div");
    this.mapElement.className = "maplibre-route-map";
    this.mapElement.addEventListener(
      "pointerdown",
      this.markUserMapInteraction,
    );
    this.mapElement.addEventListener("wheel", this.markUserMapInteraction);
    this.mapElement.addEventListener("touchstart", this.markUserMapInteraction);
    this.mapElement.addEventListener("dblclick", this.markUserMapInteraction);
    this.mapElement.addEventListener("keydown", this.markUserMapInteraction);
    this.container.prepend(this.mapElement);
    this.initPromise = this.initMap().catch(() => {
      if (!this.destroyed) this.callbacks.onError?.();
    });
    this.resizeObserver = new ResizeObserver(() => {
      if (!this.map || this.destroyed) return;
      this.map.resize();
      if (this.state.viewportMode === "fit_route") {
        this.fitBoundsForPoints(allVisiblePoints(this.state));
      }
    });
    this.resizeObserver.observe(this.container);
  }

  render(state: RouteMapState): void {
    this.state = state;
    this.container.dataset.viewportMode = state.viewportMode;
    this.container.dataset.pointCount = String(countPoints(state));
    this.container.dataset.memberCount = String(state.members.length);

    if (!this.mapReady) {
      void this.initPromise.then(() => this.renderCurrentState());
      return;
    }

    this.renderCurrentState();
  }

  fitToRoute(): void {
    this.state = {
      ...this.state,
      focusedMemberId: undefined,
      viewportMode: "fit_route",
    };
    this.callbacks.onViewportChanged?.("fit_route");
    this.render(this.state);
    this.fitBoundsForPoints(allVisiblePoints(this.state));
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

    const member = this.state.members.find(
      (candidate) => candidate.id === memberId,
    );
    if (member) {
      this.fitBoundsForPoints(memberVisiblePoints(member));
    }
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
    this.destroyed = true;
    this.resizeObserver.disconnect();
    delete this.container.dataset.viewportMode;
    delete this.container.dataset.pointCount;
    delete this.container.dataset.memberCount;
    this.mapElement.removeEventListener(
      "pointerdown",
      this.markUserMapInteraction,
    );
    this.mapElement.removeEventListener("wheel", this.markUserMapInteraction);
    this.mapElement.removeEventListener(
      "touchstart",
      this.markUserMapInteraction,
    );
    this.mapElement.removeEventListener(
      "dblclick",
      this.markUserMapInteraction,
    );
    this.mapElement.removeEventListener("keydown", this.markUserMapInteraction);

    if (this.userMapInteractionReset) {
      clearTimeout(this.userMapInteractionReset);
      this.userMapInteractionReset = null;
    }

    for (const label of this.labels.values()) label.marker.remove();
    this.labels.clear();
    if (this.map) {
      this.map.off("move", this.layoutLabels);
      this.map.off("resize", this.layoutLabels);
      this.map.remove();
      this.map = null;
    } else {
      this.mapElement.remove();
    }
  }

  private async initMap(): Promise<void> {
    const maplibregl = await import("maplibre-gl");

    if (this.destroyed) {
      return;
    }

    this.map = new maplibregl.Map({
      container: this.mapElement,
      style: routeMapStyle,
      center: [14.5058, 46.0569],
      zoom: 11,
      attributionControl: false,
    });

    this.createLabelMarker = (element) => new maplibregl.Marker({
      element, anchor: "bottom", offset: [0, -memberLabelOffset],
    });
    this.map.on("move", this.layoutLabels);
    this.map.on("resize", this.layoutLabels);

    this.map.addControl(
      new maplibregl.AttributionControl({ compact: true }),
      "bottom-left",
    );

    this.map.on("dragstart", (event) => this.handleMapInteraction(event));
    this.map.on("zoomstart", (event) => this.handleMapInteraction(event));
    this.map.on("rotatestart", (event) => this.handleMapInteraction(event));
    this.map.on("pitchstart", (event) => this.handleMapInteraction(event));

    this.map.on("error", () => {
      if (!this.destroyed) this.callbacks.onError?.();
    });

    this.map.on("load", () => {
      if (!this.map || this.destroyed) {
        return;
      }

      this.addRouteLayers();
      this.mapReady = true;
      this.callbacks.onReady?.();
      this.renderCurrentState();
    });
  }

  private renderCurrentState(): void {
    if (!this.map || !this.mapReady || this.destroyed) {
      return;
    }

    this.setSourceData(pathSourceId, pathFeatures(this.state));
    this.setSourceData(markerSourceId, markerFeatures(this.state));
    this.renderLabels();

    if (this.state.viewportMode === "fit_route") {
      this.fitBoundsForPoints(allVisiblePoints(this.state));
    } else if (this.state.focusedMemberId) {
      const member = this.state.members.find(
        (candidate) => candidate.id === this.state.focusedMemberId,
      );
      if (member) {
        this.fitBoundsForPoints(memberVisiblePoints(member));
      }
    }
  }

  private renderLabels(): void {
    if (!this.map || !this.createLabelMarker) return;
    const located = this.state.members.filter((member) => member.latestPoint && isValidPoint(member.latestPoint));
    const ids = new Set(located.map((member) => member.id));
    for (const [id, label] of this.labels) {
      if (!ids.has(id)) { label.marker.remove(); this.labels.delete(id); }
    }
    for (const member of located) {
      let label = this.labels.get(member.id);
      if (!label) {
        const element = document.createElement("button");
        element.type = "button";
        element.className = "map-member-label";
        const name = document.createElement("span");
        name.className = "map-member-label-name";
        const status = document.createElement("span");
        status.className = "map-member-label-status";
        element.append(name, status);
        element.addEventListener("click", (event) => { event.stopPropagation(); this.fitToMember(member.id); });
        label = { element, marker: this.createLabelMarker(element).setLngLat(pointToCoordinates(member.latestPoint!)).addTo(this.map) };
        this.labels.set(member.id, label);
      }
      const status = memberLocationStatus(member.status);
      label.element.children[0].textContent = member.displayName;
      label.element.children[1].textContent = status;
      (label.element.children[1] as HTMLElement).hidden = !status;
      const description = `${member.displayName}, ${status || "Sharing location"}. Last location ${new Date(member.latestPoint!.recordedAt).toLocaleString()}`;
      label.element.setAttribute("aria-label", description);
      label.element.title = description;
      label.element.classList.toggle("is-selected", member.id === this.state.focusedMemberId);
      label.marker.setLngLat(pointToCoordinates(member.latestPoint!));
    }
    this.layoutLabels();
  }

  private layoutLabels = (): void => {
    if (!this.map || this.destroyed) return;
    const visible = new Set(visibleMemberLabels([...this.labels].map(([id, label]) => {
      const point = this.map!.project(label.marker.getLngLat());
      return { id, x: point.x, y: point.y, width: label.element.offsetWidth, height: label.element.offsetHeight };
    }), { width: this.mapElement.clientWidth, height: this.mapElement.clientHeight, top: this.state.focusedMemberId ? 150 : 48, bottom: 32 }, this.state.focusedMemberId));
    for (const [id, label] of this.labels) {
      label.element.style.visibility = visible.has(id) ? "visible" : "hidden";
      label.element.tabIndex = visible.has(id) ? 0 : -1;
    }
  };

  private addRouteLayers(): void {
    if (!this.map) {
      return;
    }

    this.map.addSource(pathSourceId, {
      type: "geojson",
      data: emptyFeatureCollection(),
    });

    this.map.addSource(markerSourceId, {
      type: "geojson",
      data: emptyFeatureCollection(),
    });

    this.map.addLayer({
      id: pathLayerId,
      type: "line",
      source: pathSourceId,
      layout: {
        "line-cap": "round",
        "line-join": "round",
      },
      paint: {
        "line-color": ["coalesce", ["get", "color"], "#22c55e"],
        "line-opacity": 0.88,
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 3, 15, 6],
      },
    });

    this.map.addLayer({
      id: markerHaloLayerId,
      type: "circle",
      source: markerSourceId,
      paint: {
        "circle-color": "#071013",
        "circle-opacity": 0.86,
        "circle-radius": ["interpolate", ["linear"], ["zoom"], 10, 9, 15, 14],
      },
    });

    this.map.addLayer({
      id: markerLayerId,
      type: "circle",
      source: markerSourceId,
      paint: {
        "circle-color": ["coalesce", ["get", "color"], "#22c55e"],
        "circle-stroke-color": "#f8fafc",
        "circle-stroke-width": 2,
        "circle-radius": ["interpolate", ["linear"], ["zoom"], 10, 6, 15, 9],
      },
    });

    this.map.on("click", markerLayerId, (event) =>
      this.handleMarkerClick(event),
    );
    this.map.on("mouseenter", markerLayerId, () => {
      if (this.map) {
        this.map.getCanvas().style.cursor = "pointer";
      }
    });
    this.map.on("mouseleave", markerLayerId, () => {
      if (this.map) {
        this.map.getCanvas().style.cursor = "";
      }
    });
  }

  private setSourceData(
    sourceId: string,
    data: GeoJsonFeatureCollection,
  ): void {
    const source = this.map?.getSource(sourceId) as GeoJSONSource | undefined;
    source?.setData(data);
  }

  private fitBoundsForPoints(points: RouteMapPoint[]): void {
    if (!this.map || points.length === 0) {
      return;
    }

    const bounds = boundsForPoints(points);
    if (!bounds) {
      return;
    }

    this.map.fitBounds(bounds, {
      maxZoom: points.length === 1 ? 15 : 16,
      padding: { top: this.state.focusedMemberId ? 220 : 120, right: 90, bottom: 58, left: 90 },
      duration: window.matchMedia("(prefers-reduced-motion: reduce)").matches
        ? 0
        : 350,
    });
  }

  private handleMapInteraction(event: MapInteractionEvent): void {
    if (!event.originalEvent && !this.pendingUserMapInteraction) {
      return;
    }

    if (this.state.viewportMode === "manual" && !this.state.focusedMemberId) {
      return;
    }

    this.state = {
      ...this.state,
      focusedMemberId: undefined,
      viewportMode: "manual",
    };
    this.callbacks.onMapInteraction?.();
    this.callbacks.onViewportChanged?.("manual");
  }

  private readonly markUserMapInteraction = (event: Event): void => {
    if (event.type !== "wheel" && (event.target as HTMLElement).closest?.(".map-member-label")) return;
    this.pendingUserMapInteraction = true;

    if (this.userMapInteractionReset) {
      clearTimeout(this.userMapInteractionReset);
    }

    this.userMapInteractionReset = setTimeout(() => {
      this.pendingUserMapInteraction = false;
      this.userMapInteractionReset = null;
    }, 1000);
  };

  private handleMarkerClick(event: MapLayerMouseEvent): void {
    const memberId = event.features?.[0]?.properties?.memberId;
    if (typeof memberId === "string") {
      this.fitToMember(memberId);
    }
  }
}

function pathFeatures(state: RouteMapState): GeoJsonFeatureCollection {
  return {
    type: "FeatureCollection",
    features: state.members.flatMap((member) =>
      member.paths
        .filter((path) => path.points.length >= 2)
        .map((path) => ({
          type: "Feature" as const,
          properties: {
            memberId: member.id,
            pathId: path.id ?? null,
            color: member.color,
          },
          geometry: {
            type: "LineString" as const,
            coordinates: path.points.map(pointToCoordinates),
          },
        })),
    ),
  };
}

function markerFeatures(state: RouteMapState): GeoJsonFeatureCollection {
  return {
    type: "FeatureCollection",
    features: state.members
      .filter((member) => member.latestPoint)
      .map((member) => ({
        type: "Feature" as const,
        properties: {
          memberId: member.id,
          displayName: member.displayName,
          color: member.color,
          status: member.status,
          transportMode: member.transportMode,
        },
        geometry: {
          type: "Point" as const,
          coordinates: pointToCoordinates(member.latestPoint as RouteMapPoint),
        },
      })),
  };
}

function allVisiblePoints(state: RouteMapState): RouteMapPoint[] {
  return state.members.flatMap(memberVisiblePoints);
}

function memberVisiblePoints(member: RouteMapMember): RouteMapPoint[] {
  return [
    ...member.paths.flatMap((path) => path.points),
    ...(member.latestPoint ? [member.latestPoint] : []),
  ];
}

function boundsForPoints(points: RouteMapPoint[]): LngLatBoundsLike | null {
  const validPoints = points.filter(isValidPoint);
  if (validPoints.length === 0) {
    return null;
  }

  const longitudes = validPoints.map((point) => point.longitude);
  const latitudes = validPoints.map((point) => point.latitude);

  return [
    [Math.min(...longitudes), Math.min(...latitudes)],
    [Math.max(...longitudes), Math.max(...latitudes)],
  ];
}

function pointToCoordinates(point: RouteMapPoint): [number, number] {
  return [point.longitude, point.latitude];
}

function isValidPoint(point: RouteMapPoint): boolean {
  return (
    Number.isFinite(point.latitude) &&
    Number.isFinite(point.longitude) &&
    point.latitude >= -90 &&
    point.latitude <= 90 &&
    point.longitude >= -180 &&
    point.longitude <= 180
  );
}

function emptyFeatureCollection(): GeoJsonFeatureCollection {
  return {
    type: "FeatureCollection",
    features: [],
  };
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
