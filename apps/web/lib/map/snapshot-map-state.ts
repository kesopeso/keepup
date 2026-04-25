import type { RouteSnapshot } from "../routes-api";
import type { RouteMapMember, RouteMapPoint, RouteMapState } from "./route-map-types";

export function routeSnapshotToMapState(snapshot: RouteSnapshot): RouteMapState {
  return {
    members: snapshot.members.map((member): RouteMapMember => {
      const paths = member.paths.map((path) => ({
        id: path.id,
        points: path.points.map((point): RouteMapPoint => ({
          latitude: point.latitude,
          longitude: point.longitude,
          recordedAt: point.recordedAt,
        })),
      }));
      const latestPoint = latestPointFromPaths(paths);

      return {
        id: member.id,
        displayName: member.displayName,
        color: member.color,
        status: member.status,
        transportMode: member.transportMode,
        paths,
        latestPoint,
      };
    }),
    viewportMode: "fit_route",
  };
}

function latestPointFromPaths(
  paths: Array<{ points: RouteMapPoint[] }>,
): RouteMapPoint | undefined {
  return paths
    .flatMap((path) => path.points)
    .sort(
      (first, second) =>
        new Date(second.recordedAt).getTime() -
        new Date(first.recordedAt).getTime(),
    )[0];
}
