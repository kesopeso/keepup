const assert = require('node:assert/strict');
const { test } = require('node:test');
const { createForegroundSharing, LocationAccessError, positionPayload } = require('../src/location/foreground-sharing.ts');
const flush = () => new Promise(resolve => setImmediate(resolve));
const fix = (timestamp = Date.now(), accuracy = 5) => ({ timestamp, coords: { latitude: 46, longitude: 14, accuracy, altitude: null, speed: -1, heading: -1 } });
const snapshot = (status = 'spectating', routeStatus = 'active') => ({ route: { status: routeStatus }, viewer: { memberId: 'me', status,
  canStartSharing: ['spectating','stale'].includes(status), canStopSharing: ['tracking','stale'].includes(status) } });
function harness(t, overrides = {}) {
  const commands = [], positions = [], states = [], watches = [];
  const capture = createForegroundSharing({ prepare: async () => fix(),
    watch: async (sample, error) => { const watch = { sample, error, removed: false, remove() { this.removed = true; } }; watches.push(watch); return watch; },
    command: async type => { commands.push(type); }, send: payload => { positions.push(payload); return true; },
    ...overrides, onState: state => states.push(state) });
  capture.update(snapshot()); capture.connection('live'); t.after(() => capture.dispose());
  return { capture, commands, positions, states, watches };
}
async function start(h) { await h.capture.start(); h.capture.update(snapshot('tracking')); await flush(); }

test('location payload includes measured accuracy and timestamp and omits unusable optional values', () => {
  const value = fix(1000); assert.deepEqual(positionPayload(value), { latitude:46,longitude:14,accuracyM:5,clientRecordedAt:'1970-01-01T00:00:01.000Z' });
  for(const sample of [fix(NaN),fix(1000,null),fix(1000,-1),{...fix(),coords:{...fix().coords,latitude:91}}]) assert.equal(positionPayload(sample),null);
});
test('permission is requested only on Start, before the command, and only confirmed tracking sends locations', async t => {
  const calls=[]; const h=harness(t,{prepare:async()=>{calls.push('permission');return fix();},command:async type=>calls.push(type)});
  assert.deepEqual(calls,[]); await h.capture.start(); assert.deepEqual(calls,['permission','start_sharing']);
  assert.equal(h.positions.length,0); h.capture.update(snapshot('tracking')); await flush(); assert.equal(h.positions.length,1);
  h.watches[0].sample(fix(Date.now()+1000)); assert.equal(h.positions.length,2);
});
test('denied or blocked permission does not occupy a tracker slot and has actionable feedback', async t => {
  const h=harness(t,{prepare:async()=>{throw new LocationAccessError('Open settings',true);}}); await h.capture.start();
  assert.equal(h.commands.length,0); assert.equal(h.states.at(-1).settings,true); assert.equal(h.watches.length,0);
});
test('slot and policy rejections do not start GPS capture', async t => {
  const h=harness(t,{command:async()=>{throw new Error('tracking_limit_reached');}}); await h.capture.start();
  assert.match(h.states.at(-1).error,/slots/); assert.equal(h.states.at(-1).sharing,false); assert.equal(h.watches.length,0);
});
test('Stop removes the watcher immediately and ignores late fixes and GPS rejections', async t => {
  const h=harness(t); await start(h); const count=h.positions.length; await h.capture.stop();
  assert.equal(h.watches[0].removed,true); h.watches[0].sample(fix(Date.now()+1000)); h.capture.event({type:'position_rejected',error:'accuracy_too_low'});
  assert.equal(h.positions.length,count); assert.equal(h.states.at(-1).error,null); assert.equal(h.commands.at(-1),'stop_sharing');
});
test('a failed Stop still stops local capture and offers stale recovery', async t => {
  let fail=false;const h=harness(t,{command:async()=>{if(fail)throw new Error('network unavailable');}});await start(h);fail=true;
  assert.equal(await h.capture.stop(),false);assert.equal(h.watches[0].removed,true);assert.equal(h.states.at(-1).recovery,true);
});
test('GPS errors persist through other members updates and clear on own acceptance', async t => {
  const h=harness(t);await start(h);h.capture.event({type:'position_rejected',error:'accuracy_too_low'});
  const event={type:'position_updated',memberId:'other',segmentId:'segment',point:{seq:1,latitude:46,longitude:14,recordedAt:new Date().toISOString()}};
  h.capture.event(event); h.capture.update(snapshot('tracking')); assert.match(h.states.at(-1).error,/accuracy/);
  h.capture.event({...event,memberId:'me'});assert.equal(h.states.at(-1).error,null);
});
test('socket interruption drops fixes and restarts capture after catch-up without buffering', async t => {
  const h=harness(t);await start(h);const count=h.positions.length;h.capture.connection('reconnecting');h.watches[0].sample(fix(Date.now()+1000));
  assert.equal(h.watches[0].removed,true);assert.equal(h.positions.length,count);h.capture.update(snapshot('stale'));h.capture.connection('live');await flush();
  assert.equal(h.watches.length,2);assert.equal(h.positions.length,count);
});
test('background requires explicit resume or spectator choice and cancels pending permission', async t => {
  const h=harness(t);await start(h);h.capture.connection('paused');h.capture.update(snapshot('stale'));h.capture.connection('live');await flush();
  assert.equal(h.states.at(-1).recovery,true);assert.equal(h.watches.length,1);await start(h);assert.equal(h.watches.length,2);
  let resolve;const pending=harness(t,{prepare:()=>new Promise(done=>resolve=done)});const starting=pending.capture.start();await flush();pending.capture.connection('paused');resolve(fix());await starting;
  assert.equal(pending.commands.length,0);assert.equal(pending.states.at(-1).action,null);
});
test('late native watch installation is removed after Stop or disposal', async t => {
  let resolve;const h=harness(t,{watch:()=>new Promise(done=>resolve=done)});await h.capture.start();h.capture.update(snapshot('tracking'));h.capture.dispose();
  let removed=false;resolve({remove:()=>removed=true});await flush();assert.equal(removed,true);
});
test('closed routes release capture and restored tracking never automatically captures GPS', async t => {
  const h=harness(t);h.capture.update(snapshot('tracking'));assert.equal(h.states.at(-1).recovery,true);assert.equal(h.watches.length,0);
  await start(h);h.capture.update(snapshot('spectating','closed'));assert.equal(h.watches[0].removed,true);assert.equal(h.states.at(-1).sharing,false);
});
test('watcher failures stop capture and release the server tracking slot', async t => {
  const h=harness(t);await start(h);h.watches[0].error('Device location disabled');await flush();
  assert.equal(h.states.at(-1).sharing,false);assert.match(h.states.at(-1).error,/disabled/);assert.equal(h.commands.at(-1),'stop_sharing');
});


test('Android permission-dialog pause reports denial and waits for foreground before obtaining GPS', async t => {
  let reject; const denied = harness(t, { requestPermission: () => new Promise((_, fail) => reject = fail) });
  const attempt = denied.capture.start(); denied.capture.connection('paused');
  reject(new LocationAccessError('Permission denied')); await attempt;
  assert.equal(denied.states.at(-1).error, 'Permission denied'); assert.equal(denied.commands.length, 0);
  let resolve; let prepared = false;
  const allowed = harness(t, { requestPermission: () => new Promise(done => resolve = done), prepare: async () => { prepared = true; return fix(); } });
  const starting = allowed.capture.start(); allowed.capture.connection('paused'); resolve(); await flush();
  assert.equal(prepared, false); allowed.capture.connection('live'); await starting;
  assert.equal(prepared, true); assert.equal(allowed.commands.at(-1), 'start_sharing');
});


test('Stop while disconnected cancels sharing intent and never restarts GPS on reconnection', async t => {
  const h = harness(t); await start(h); h.capture.connection('reconnecting');
  assert.equal(await h.capture.stop(), false); assert.equal(h.states.at(-1).sharing, false);
  h.capture.update(snapshot('stale')); h.capture.connection('live'); await flush();
  assert.equal(h.watches.length, 1); assert.equal(h.states.at(-1).recovery, true);
});
test('leaving the foreground aborts an initial GPS fix and ignores a late result', async t => {
  let signal, resolve; const h = harness(t, { prepare: value => { signal = value; return new Promise(done => resolve = done); } });
  const pending = h.capture.start(); await flush(); h.capture.connection('paused');
  assert.equal(signal.aborted, true); resolve(fix()); await pending; assert.equal(h.commands.length, 0);
});
