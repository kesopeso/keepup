const assert = require('node:assert/strict');
const { test } = require('node:test');
const { createViewingSession, viewingSocketUrl, applyPosition, parsePosition } = require('../src/live/viewing-session.ts');
const member = { code: 'ABC123', memberId: 'viewer', memberToken: 'secret' };
const base = () => ({ route: { code: 'ABC123', status: 'active' }, viewer: { memberId: 'viewer' },
  members: [{ id: 'owner', paths: [{ id: 'segment', points: [] }] }] });
const position = (seq = 1) => ({ type: 'position_updated', memberId: 'owner', segmentId: 'segment',
  point: { seq, latitude: 46, longitude: 14 + seq / 1000, recordedAt: '2026-10-06T12:00:00Z' } });
const flush = () => new Promise((resolve) => setImmediate(resolve));
class Socket {
  listeners = {}; sent = []; closed = false;
  addEventListener(type, callback) { (this.listeners[type] ??= []).push(callback); }
  send(value) { this.sent.push(JSON.parse(value)); }
  close() { this.closed = true; this.emit('close', {}); }
  emit(type, value) { for (const callback of this.listeners[type] ?? []) callback(value); }
  message(value) { this.emit('message', { data: JSON.stringify(value) }); }
}
function harness(t, load = async () => base()) {
  const sockets = [], snapshots = [], statuses = [], errors = [], timers = [];
  const session = createViewingSession({ url: 'ws://localhost/ws', member, load,
    createSocket: () => { const socket = new Socket(); sockets.push(socket); return socket; },
    onSnapshot: (value) => snapshots.push(value), onStatus: (value) => statuses.push(value),
    onError: (message, invalid) => errors.push({ message, invalid }), isInvalid: (error) => error.invalid === true,
    schedule: (callback, delay) => { const timer = { callback, delay }; timers.push(timer); return timer; },
    cancel: (timer) => { timer.cancelled = true; },
  });
  t.after(() => session.stop());
  return { session, sockets, snapshots, statuses, errors, timers };
}
async function established(h) {
  await flush(); h.sockets.at(-1).emit('open', {});
  h.sockets.at(-1).message({ type: 'connection_established' }); await flush();
}

test('WebSocket URLs preserve deployment prefixes and use secure transport with HTTPS', () => {
  assert.equal(viewingSocketUrl('http://10.0.2.2:3000/api'), 'ws://10.0.2.2:3000/ws');
  assert.equal(viewingSocketUrl('https://host/keepup/api'), 'wss://host/keepup/ws');
  assert.equal(viewingSocketUrl('http://host/'), 'ws://host/ws');
});
test('position validation rejects malformed geometry, timestamps and sequence numbers', () => {
  for (const point of [{ latitude: 91 }, { longitude: Infinity }, { recordedAt: 'bad' }, { seq: 0 }, { seq: 1.5 }]) {
    assert.equal(parsePosition({ ...position(), point: { ...position().point, ...point } }), null);
  }
  assert.equal(parsePosition(null), null);
});
test('positions remain ordered and deduplicated without joining separate segments or mutating history', () => {
  const original = base(); let snapshot = applyPosition(original, position(2));
  snapshot = applyPosition(snapshot, position(1)); snapshot = applyPosition(snapshot, position(2));
  assert.deepEqual(snapshot.members[0].paths[0].points.map((point) => point.seq), [1, 2]);
  assert.equal(original.members[0].paths[0].points.length, 0);
  assert.equal(applyPosition(snapshot, { ...position(), segmentId: 'other' }).members[0].paths.length, 1);
});
test('saved token is sent only in authentication and positions update without another request', async (t) => {
  let calls = 0; const h = harness(t, async () => { calls++; return base(); }); await established(h);
  assert.deepEqual(h.sockets[0].sent, [{ type: 'authenticate', memberToken: 'secret' }]);
  const before = calls; h.sockets[0].message(position());
  assert.equal(calls, before); assert.equal(h.snapshots.at(-1).members[0].paths[0].points.length, 1);
  assert.equal(h.statuses.at(-1), 'live');
});
test('snapshot loading buffers concurrent positions and deduplicates already persisted points', async (t) => {
  let resolve; let calls = 0;
  const h = harness(t, async () => ++calls === 1 ? base() : new Promise((done) => { resolve = done; }));
  await flush(); h.sockets[0].message({ type: 'connection_established' });
  h.sockets[0].message(position(1)); h.sockets[0].message(position(2));
  resolve(applyPosition(base(), position(1))); await flush();
  assert.deepEqual(h.snapshots.at(-1).members[0].paths[0].points.map((point) => point.seq), [1, 2]);
});
test('membership and route changes resync, including events received during a request', async (t) => {
  let calls = 0, resolve;
  const h = harness(t, async () => ++calls === 3 ? new Promise((done) => { resolve = done; }) : base());
  await established(h); h.sockets[0].message({ type: 'member_joined' });
  h.sockets[0].message({ type: 'route_updated' }); resolve(base()); await flush();
  assert.equal(calls, 4);
});
test('unexpected close backs off, resyncs missed points, and ignores obsolete socket events', async (t) => {
  let current = base(); const h = harness(t, async () => structuredClone(current)); await established(h);
  const old = h.sockets[0]; old.emit('close', { code: 1006 });
  const timer = h.timers.find((timer) => !timer.cancelled && timer.delay === 1000);
  assert.ok(timer); current = applyPosition(current, position()); timer.callback(); await flush();
  const watchdog = h.timers.at(-1);
  old.emit('close', { code: 1006 }); assert.ok(!watchdog.cancelled);
  await established(h);
  assert.equal(h.snapshots.at(-1).members[0].paths[0].points.length, 1);
  old.message(position(2)); assert.equal(h.snapshots.at(-1).members[0].paths[0].points.length, 1);
});
test('background pauses and foreground catches up; stopping prevents further callbacks', async (t) => {
  const h = harness(t); await established(h); h.session.setForeground(false);
  assert.equal(h.sockets[0].closed, true); assert.equal(h.statuses.at(-1), 'paused');
  h.session.setForeground(true); await established(h); assert.equal(h.sockets.length, 2);
  h.session.stop(); const count = h.snapshots.length; h.sockets[1].message(position()); await flush();
  assert.equal(h.snapshots.length, count);
});
test('closed archives do not connect and route closure stops an existing connection', async (t) => {
  let current = base(); const h = harness(t, async () => current); await established(h);
  current = { ...base(), route: { code: 'ABC123', status: 'closed' } };
  h.sockets[0].message({ type: 'route_closed' }); await flush();
  assert.equal(h.sockets[0].closed, true); assert.equal(h.statuses.at(-1), 'archive');
  const archive = harness(t, async () => current); await flush(); assert.equal(archive.sockets.length, 0);
});
test('network failures retain history and retry, while invalid credentials stop recovery', async (t) => {
  let error; const h = harness(t, async () => { if (error) throw error; return base(); }); await established(h);
  error = new Error('network unavailable'); await h.session.refresh();
  assert.ok(h.snapshots.length); assert.equal(h.statuses.at(-1), 'reconnecting');
  error = Object.assign(new Error('invalid credentials'), { invalid: true }); await h.session.refresh();
  assert.equal(h.statuses.at(-1), 'invalid'); assert.equal(h.errors.at(-1).invalid, true);
  const count = h.sockets.length; h.session.setForeground(false); h.session.setForeground(true); await flush();
  assert.equal(h.sockets.length, count);
});
test('duplicate membership rejection retries, and authentication stalls have a timeout', async (t) => {
  const h = harness(t); await flush(); h.sockets[0].message({ type: 'live_connection_rejected', reason: 'already_active_connection' });
  assert.equal(h.statuses.at(-1), 'reconnecting'); assert.match(h.errors.at(-1).message, /connected elsewhere/);
  const stalled = harness(t); await flush(); stalled.timers.find((timer) => timer.delay === 10000).callback();
  assert.equal(stalled.sockets[0].closed, true); assert.equal(stalled.statuses.at(-1), 'reconnecting');
});
