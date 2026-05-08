"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { createRouteMapRenderer } from "../../lib/map/route-map-renderer-factory";
import type {
  RouteMapRenderer,
  RouteMapState,
  RouteMapViewportMode,
} from "../../lib/map/route-map-types";

export function RouteMap({ state }: { state: RouteMapState }) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const rendererRef = useRef<RouteMapRenderer | null>(null);
  const [viewportMode, setViewportMode] = useState<RouteMapViewportMode>(
    state.viewportMode,
  );
  const renderedState = useMemo(
    () => ({
      ...state,
      viewportMode,
    }),
    [state, viewportMode],
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
        <div className="map-state">
          <strong>{pointCount}</strong>
          <span>{pointCount === 1 ? "point" : "points"}</span>
        </div>
        <div className="map-tools" aria-label="Map controls">
          <button
            onClick={() => {
              setViewportMode("fit_route");
              rendererRef.current?.fitToRoute();
            }}
            type="button"
          >
            Fit
          </button>
          <span>{viewportMode === "fit_route" ? "Auto" : "Manual"}</span>
        </div>
      </div>
    </section>
  );
}
