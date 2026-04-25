import type { RouteSnapshot } from "../routes-api";
import type {
  RouteMapMember,
  RouteMapPoint,
  RouteMapState,
} from "./route-map-types";

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

export function appendLiveRoutePoint(
  state: RouteMapState,
  update: {
    memberId: string;
    segmentId?: string;
    point: RouteMapPoint;
  },
): RouteMapState {
  let memberFound = false;

  const members = state.members.map((member) => {
    if (member.id !== update.memberId) {
      return member;
    }

    memberFound = true;
    const pathIndex = member.paths.findIndex(
      (path) => path.id === update.segmentId,
    );

    const paths =
      pathIndex >= 0
        ? member.paths.map((path, index) =>
            index === pathIndex
              ? { ...path, points: [...path.points, update.point] }
              : path,
          )
        : [
            ...member.paths,
            {
              id: update.segmentId,
              points: [update.point],
            },
          ];

    return {
      ...member,
      paths,
      latestPoint: update.point,
    };
  });

  if (!memberFound) {
    return state;
  }

  return {
    ...state,
    members,
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
