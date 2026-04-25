import { PlaceholderRouteMapRenderer } from "./placeholder-route-map-renderer";
import type { RouteMapRendererFactory } from "./route-map-types";

export const createRouteMapRenderer: RouteMapRendererFactory = (
  container,
  callbacks,
) => new PlaceholderRouteMapRenderer(container, callbacks);
