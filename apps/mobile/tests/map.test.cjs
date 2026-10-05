const assert = require('node:assert/strict');
const { test } = require('node:test');
const { snapshotGeometry, boundsForPoints } = require('../src/map/snapshot-geometry.ts');
const { createRoutesApi } = require('../src/api/routes.ts');
const point = (longitude, latitude = 46, recordedAt = '2026-10-05T08:00:00Z') => ({ longitude, latitude, recordedAt });
const member = (id, paths, status = 'tracking') => ({ id, displayName: id, color: '#22c55e', status, paths });

test('map preserves segment gaps, member colors, and latest timestamps across unordered paths', () => {
  const snapshot = { members: [member('one', [
    { points: [point(14), point(14.01, 46.01, '2026-10-05T08:05:00Z')] },
    { points: [point(15, 47, '2026-10-04T08:00:00Z'), point(15.01, 47.01, '2026-10-04T08:05:00Z')] },
  ]), member('two', [{ points: [point(16)] }], 'offline'), member('empty', [])] };
  const geometry = snapshotGeometry(snapshot);
  assert.equal(geometry.paths.features.length, 2);
  assert.deepEqual(geometry.paths.features[0].geometry.coordinates, [[14, 46], [14.01, 46.01]]);
  assert.equal(geometry.markers.features.length, 2);
  assert.deepEqual(geometry.markers.features[0].geometry.coordinates, [14.01, 46.01]);
  assert.equal(geometry.markers.features[1].properties.status, 'offline');
  assert.equal(geometry.paths.features[0].properties.color, '#22c55e');
  assert.equal(geometry.members[2].latest, null);
  assert.deepEqual(geometry.bounds, [14, 46, 16, 47.01]);
});

test('empty and single-point maps have usable bounds without fake locations', () => {
  assert.equal(snapshotGeometry({ members: [member('empty', [])] }).bounds, null);
  assert.deepEqual(boundsForPoints([point(14, 46)]), [14, 46, 14, 46]);
});

test('invalid points break lines, never create false markers, and never mutate the snapshot', () => {
  const snapshot = { members: [member('one', [{ points: [point(14), point(NaN), point(15), point(16)] }])] };
  const original = structuredClone(snapshot);
  const geometry = snapshotGeometry(snapshot);
  assert.deepEqual(geometry.paths.features[0].geometry.coordinates, [[15, 46], [16, 46]]);
  assert.equal(geometry.paths.features.length, 1);
  assert.deepEqual(snapshot, original);
  for (const invalid of [point(181), point(14, 91), point(14, 46, 'invalid')]) assert.equal(boundsForPoints([invalid]), null);
});

test('paths and fit bounds cross the antimeridian over the short arc', () => {
  const geometry = snapshotGeometry({ members: [member('one', [{ points: [point(179.8), point(-179.8)] }])] });
  assert.deepEqual(geometry.paths.features[0].geometry.coordinates, [[179.8, 46], [180.2, 46]]);
  assert.deepEqual(geometry.bounds, [179.8, 46, 180.2, 46]);
  assert.deepEqual(boundsForPoints([point(0, 90)]), [0, 85, 0, 85]);
});

test('API rejects malformed path structures and coordinates before native rendering', async (t) => {
  const base = { route: { code: 'ABC123', name: 'Route', description: '', status: 'active', sharingPolicy: 'everyone_can_share' },
    viewer: { memberId: 'one' }, members: [{ ...member('one', []), transportMode: 'walking', role: 'member' }] };
  for (const paths of [[null], [{}], [{ points: {} }], [{ points: [point(181)] }], [{ points: [point(14, 46, 'invalid')] }]]) {
    t.mock.method(global, 'fetch', async () => Response.json({ ...base, members: [{ ...base.members[0], paths }] }));
    await assert.rejects(createRoutesApi('https://example.com/api').getSnapshot('ABC123', 'token', new AbortController().signal),
      (error) => error.code === 'invalid_response');
  }
});
