import { MapLibreRouteMapRenderer } from "./maplibre-route-map-renderer";
import type { RouteMapRendererFactory } from "./route-map-types";

export const createRouteMapRenderer: RouteMapRendererFactory = (
  container,
  callbacks,
) => new MapLibreRouteMapRenderer(container, callbacks);
