"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { memberLocationStatus } from "../../lib/map/member-labels";
import { createRouteMapRenderer } from "../../lib/map/route-map-renderer-factory";
import type {
  RouteMapRenderer,
  RouteMapState,
  RouteMapViewportMode,
} from "../../lib/map/route-map-types";

export function RouteMap({
  state,
  archived,
  sharingCount,
}: {
  state: RouteMapState;
  archived: boolean;
  sharingCount: number;
}) {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const selected = state.members.find((member) => member.id === selectedId && member.latestPoint);
  const [mapError, setMapError] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const rendererRef = useRef<RouteMapRenderer | null>(null);
  const [viewportMode, setViewportMode] = useState<RouteMapViewportMode>(
    state.viewportMode,
  );
  const renderedState = useMemo(
    () => ({
      ...state,
      viewportMode,
      focusedMemberId: selected?.id,
    }),
    [state, viewportMode, selected?.id],
  );

  const pointCount = useMemo(
    () =>
      state.members.reduce(
        (total, member) =>
          total +
          member.paths.reduce(
            (memberTotal, path) => memberTotal + path.points.length,
            0,
          ),
        0,
      ),
    [state],
  );

  useEffect(() => {
    if (!containerRef.current) {
      return;
    }

    const renderer = createRouteMapRenderer(containerRef.current, {
      onViewportChanged: setViewportMode,
      onMemberMarkerClick: setSelectedId,
      onMapInteraction: () => setSelectedId(null),
      onError: () => setMapError(true),
      onReady: () => setMapError(false),
    });
    rendererRef.current = renderer;

    return () => {
      renderer.destroy();
      rendererRef.current = null;
    };
  }, []);

  useEffect(() => {
    rendererRef.current?.render(renderedState);
  }, [renderedState]);

  return (
    <section className="map-stage" aria-label="Route map">
      <div className="map-surface" ref={containerRef}>
        <div className="map-summary">
          {archived
            ? "Saved route history"
            : `${sharingCount} sharing · ${state.members.length} ${state.members.length === 1 ? "member" : "members"}`}
        </div>
        {selected?.latestPoint && !mapError ? (
          <div className="map-member-detail" role="status">
            <strong>{selected.displayName} · {memberLocationStatus(selected.status) || "Sharing location"}</strong>
            <span>Last location {new Date(selected.latestPoint.recordedAt).toLocaleString()}</span>
          </div>
        ) : null}
        {mapError || pointCount === 0 ? (
          <div className="map-empty" role="status">
            <strong>
              {mapError
                ? "Map unavailable"
                : archived
                  ? "No recorded locations"
                  : "The route starts here"}
            </strong>
            <span>
              {mapError
                ? "Check your connection and reload to try the map again."
                : archived
                  ? "No location history was saved for this route."
                  : "Locations appear when a member starts sharing."}
            </span>
          </div>
        ) : null}
        <div className="map-tools" aria-label="Map controls">
          <button
            disabled={viewportMode === "fit_route"}
            onClick={() => {
              setSelectedId(null);
              setViewportMode("fit_route");
              rendererRef.current?.fitToRoute();
            }}
            type="button"
          >
            Fit group
          </button>
        </div>
      </div>
    </section>
  );
}
