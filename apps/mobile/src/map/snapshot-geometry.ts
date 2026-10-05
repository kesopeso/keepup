import type { RoutePoint, RouteSnapshot, SnapshotMember } from '../domain/routes.ts';

export type Coordinate = [number, number];
export type Bounds = [number, number, number, number];
type Feature<G> = { type: 'Feature'; properties: Record<string, string>; geometry: G };
type Collection<G> = { type: 'FeatureCollection'; features: Feature<G>[] };
export type MapMember = SnapshotMember & { points: RoutePoint[]; latest: RoutePoint | null };

export function validMapPoint(value: unknown): value is RoutePoint {
  if (!value || typeof value !== 'object') return false;
  const point = value as RoutePoint;
  return Number.isFinite(point.latitude) && Math.abs(point.latitude) <= 90 &&
    Number.isFinite(point.longitude) && Math.abs(point.longitude) <= 180 &&
    typeof point.recordedAt === 'string' && Number.isFinite(Date.parse(point.recordedAt));
}

export function snapshotGeometry(snapshot: RouteSnapshot) {
  const paths: Collection<{ type: 'LineString'; coordinates: Coordinate[] }> = { type: 'FeatureCollection', features: [] };
  const markers: Collection<{ type: 'Point'; coordinates: Coordinate }> = { type: 'FeatureCollection', features: [] };
  const members: MapMember[] = snapshot.members.map((member) => {
    const points: RoutePoint[] = [];
    const properties = { memberId: member.id, color: member.color, status: member.status };
    for (const segment of member.paths) {
      // Invalid samples break the line instead of drawing across an unknown gap.
      let run: Coordinate[] = [];
      const flush = () => {
        if (run.length > 1) paths.features.push({ type: 'Feature', properties, geometry: { type: 'LineString', coordinates: run } });
        run = [];
      };
      for (const point of segment.points) {
        if (!validMapPoint(point)) { flush(); continue; }
        points.push(point);
        let longitude = point.longitude;
        const previous = run.at(-1)?.[0];
        if (previous !== undefined) longitude += 360 * Math.round((previous - longitude) / 360);
        run.push([longitude, point.latitude]);
      }
      flush();
    }
    const latest = points.reduce<RoutePoint | null>((current, point) =>
      !current || Date.parse(point.recordedAt) >= Date.parse(current.recordedAt) ? point : current, null);
    if (latest) markers.features.push({ type: 'Feature', properties,
      geometry: { type: 'Point', coordinates: [latest.longitude, latest.latitude] } });
    return { ...member, points, latest };
  });
  return { paths, markers, members, bounds: boundsForPoints(members.flatMap((member) => member.points)) };
}

export function boundsForPoints(points: RoutePoint[]): Bounds | null {
  const valid = points.filter(validMapPoint);
  if (!valid.length) return null;
  // Exclude the largest empty longitude arc, so antimeridian routes fit locally.
  const longitudes = valid.map((point) => point.longitude).sort((a, b) => a - b);
  let largestGap = -1;
  let start = 0;
  for (let index = 0; index < longitudes.length; index++) {
    const gap = (longitudes[index + 1] ?? longitudes[0] + 360) - longitudes[index];
    if (gap > largestGap) { largestGap = gap; start = (index + 1) % longitudes.length; }
  }
  const west = longitudes[start];
  const east = start === 0 ? longitudes.at(-1)! : longitudes[start - 1] + 360;
  let south = 90;
  let north = -90;
  for (const point of valid) { south = Math.min(south, point.latitude); north = Math.max(north, point.latitude); }
  // MapLibre uses Web Mercator; keep camera fits away from its poles.
  return [west, Math.max(-85, Math.min(85, south)), east, Math.max(-85, Math.min(85, north))];
}
