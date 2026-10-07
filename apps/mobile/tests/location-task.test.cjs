const assert = require('node:assert/strict');
const { test } = require('node:test');
const { createLocationTask } = require('../src/location/location-task.ts');
const flush = () => new Promise(resolve => setImmediate(resolve));
const fix = timestamp => ({ timestamp, coords: { latitude: 46, longitude: 14, accuracy: 5 } });

test('task delivers only the latest fresh fix, never delayed batches', async () => {
  const samples = []; const task = createLocationTask({ start: async () => {}, stop: async () => {}, now: () => 20000 });
  await task.watch(value => samples.push(value.timestamp), assert.fail);
  await task.deliver([fix(5000), fix(19000), fix(18000)]); await task.deliver([fix(5000)]);
  await task.deliver([fix(21000)]); assert.deepEqual(samples, [19000]);
});
test('task with no sharing owner unregisters instead of restarting from saved access', async () => {
  let stops = 0, starts = 0;
  const task = createLocationTask({ start: async () => starts++, stop: async () => stops++ });
  await task.reset(); await task.deliver([fix(Date.now())]);
  assert.equal(starts, 0); assert.equal(stops, 2);
});
test('Stop clears delivery immediately and serializes native cleanup before restart', async () => {
  const calls = []; let release; let block = false; let samples = 0;
  const task = createLocationTask({ start: async () => calls.push('start'), stop: async () => {
    calls.push('stop'); if (block) { block = false; await new Promise(resolve => release = resolve); }
  } });
  const old = await task.watch(() => samples++, assert.fail); block = true; old.remove(); await flush();
  const delivery = task.deliver([fix(Date.now())]); assert.equal(samples, 0);
  const pending = task.watch(() => samples++, assert.fail); await flush();
  assert.deepEqual(calls, ['stop', 'start', 'stop']);
  release(); await delivery; const current = await pending;
  old.remove(); await task.deliver([fix(Date.now())]); assert.equal(samples, 1);
  assert.equal(task.isActive(), true); current.remove(); await flush(); assert.equal(task.isActive(), false);
});

test('native start failure releases ownership and cleans up before retry', async () => {
  let fail = true; let stops = 0;
  const task = createLocationTask({ start: async () => { if (fail) throw new Error('permission revoked'); }, stop: async () => { stops++; } });
  await assert.rejects(task.watch(assert.fail, assert.fail), /permission revoked/);
  assert.equal(task.isActive(), false); assert.equal(stops, 2);
  fail = false; const subscription = await task.watch(() => {}, assert.fail);
  assert.equal(task.isActive(), true); subscription.remove(); await flush();
});
test('native task errors reach the owner and late callbacks after removal cannot deliver', async () => {
  const errors = []; let subscription; let samples = 0;
  const task = createLocationTask({ start: async () => {}, stop: async () => {} });
  subscription = await task.watch(() => samples++, message => { errors.push(message); subscription.remove(); });
  await task.deliver([], 'permission revoked'); await task.deliver([fix(Date.now())]);
  assert.deepEqual(errors, ['permission revoked']); assert.equal(samples, 0); assert.equal(task.isActive(), false);
});
