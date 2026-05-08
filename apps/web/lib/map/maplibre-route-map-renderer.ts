import type {
  GeoJSONSource,
  LngLatBoundsLike,
  Map as MapLibreMap,
  MapLayerMouseEvent,
} from "maplibre-gl";
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
  private mapReady = false;
  private destroyed = false;
  private pendingUserMapInteraction = false;
  private userMapInteractionReset: ReturnType<typeof setTimeout> | null = null;
  private state: RouteMapState = {
    members: [],
    viewportMode: "fit_route",
  };
  private initPromise: Promise<void>;

  constructor(
    private readonly container: HTMLElement,
    private readonly callbacks: RouteMapCallbacks = {},
  ) {
    this.mapElement = document.createElement("div");
    this.mapElement.className = "maplibre-route-map";
    this.mapElement.addEventListener("pointerdown", this.markUserMapInteraction);
    this.mapElement.addEventListener("wheel", this.markUserMapInteraction);
    this.mapElement.addEventListener("touchstart", this.markUserMapInteraction);
    this.mapElement.addEventListener("dblclick", this.markUserMapInteraction);
    this.mapElement.addEventListener("keydown", this.markUserMapInteraction);
    this.container.prepend(this.mapElement);
    this.initPromise = this.initMap();
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

    const member = this.state.members.find((candidate) => candidate.id === memberId);
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
    delete this.container.dataset.viewportMode;
    delete this.container.dataset.pointCount;
    delete this.container.dataset.memberCount;
    this.mapElement.removeEventListener("pointerdown", this.markUserMapInteraction);
    this.mapElement.removeEventListener("wheel", this.markUserMapInteraction);
    this.mapElement.removeEventListener("touchstart", this.markUserMapInteraction);
    this.mapElement.removeEventListener("dblclick", this.markUserMapInteraction);
    this.mapElement.removeEventListener("keydown", this.markUserMapInteraction);

    if (this.userMapInteractionReset) {
      clearTimeout(this.userMapInteractionReset);
      this.userMapInteractionReset = null;
    }

    if (this.map) {
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

    this.map.addControl(
      new maplibregl.AttributionControl({ compact: true }),
      "bottom-left",
    );

    this.map.on("dragstart", (event) => this.handleMapInteraction(event));
    this.map.on("zoomstart", (event) => this.handleMapInteraction(event));
    this.map.on("rotatestart", (event) => this.handleMapInteraction(event));
    this.map.on("pitchstart", (event) => this.handleMapInteraction(event));

    this.map.on("load", () => {
      if (!this.map || this.destroyed) {
        return;
      }

      this.addRouteLayers();
      this.mapReady = true;
      this.renderCurrentState();
    });
  }

  private renderCurrentState(): void {
    if (!this.map || !this.mapReady || this.destroyed) {
      return;
    }

    this.setSourceData(pathSourceId, pathFeatures(this.state));
    this.setSourceData(markerSourceId, markerFeatures(this.state));

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
        "line-width": [
          "interpolate",
          ["linear"],
          ["zoom"],
          10,
          3,
          15,
          6,
        ],
      },
    });

    this.map.addLayer({
      id: markerHaloLayerId,
      type: "circle",
      source: markerSourceId,
      paint: {
        "circle-color": "#071013",
        "circle-opacity": 0.86,
        "circle-radius": [
          "interpolate",
          ["linear"],
          ["zoom"],
          10,
          9,
          15,
          14,
        ],
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
        "circle-radius": [
          "interpolate",
          ["linear"],
          ["zoom"],
          10,
          6,
          15,
          9,
        ],
      },
    });

    this.map.on("click", markerLayerId, (event) => this.handleMarkerClick(event));
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

  private setSourceData(sourceId: string, data: GeoJsonFeatureCollection): void {
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
      padding: { top: 52, right: 36, bottom: 58, left: 36 },
      duration: 350,
    });
  }

  private handleMapInteraction(event: MapInteractionEvent): void {
    if (!event.originalEvent && !this.pendingUserMapInteraction) {
      return;
    }

    if (this.state.viewportMode === "manual") {
      return;
    }

    this.state = {
      ...this.state,
      viewportMode: "manual",
    };
    this.callbacks.onMapInteraction?.();
    this.callbacks.onViewportChanged?.("manual");
  }

  private readonly markUserMapInteraction = (): void => {
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
