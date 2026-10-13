const assert = require('node:assert/strict');
const { test } = require('node:test');
const { visibleMemberLabels, memberLocationStatus } = require('../src/map/member-labels.ts');

const label = (id, x = 100, y = 120, width = 70, height = 30) => ({ id, x, y, width, height });
const viewport = { width: 400, height: 300 };

test('crowded names hide deterministically and selected members get priority without moving circles', () => {
  const labels = [label('b'), label('a'), label('c', 300)];
  const original = structuredClone(labels);
  assert.deepEqual(visibleMemberLabels(labels, viewport), ['a', 'c']);
  assert.deepEqual(visibleMemberLabels(labels, viewport, 'b'), ['b', 'c']);
  assert.deepEqual(labels, original);
  assert.deepEqual(visibleMemberLabels([label('a', 70), label('b', 190)], viewport), ['a', 'b']);
});

test('labels clear other circles and respect viewport edges and header/drawer insets', () => {
  assert.deepEqual(visibleMemberLabels([label('a'), label('b', 100, 82, 70, 30)], viewport), ['b']);
  assert.deepEqual(visibleMemberLabels([label('edge', 20), label('top', 200, 20), label('invalid', NaN)], viewport), []);
  assert.deepEqual(visibleMemberLabels([label('header', 100, 90), label('drawer', 200, 290)],
    { ...viewport, top: 60, bottom: 100 }), []);
  assert.deepEqual(visibleMemberLabels([], viewport), []);
});

test('last-known labels distinguish every non-sharing presence state', () => {
  assert.equal(memberLocationStatus('tracking'), '');
  assert.equal(memberLocationStatus('stale'), 'Location delayed');
  assert.equal(memberLocationStatus('offline'), 'Offline');
  assert.equal(memberLocationStatus('spectating'), 'Spectating');
  assert.equal(memberLocationStatus('left'), 'Left');
  assert.equal(memberLocationStatus('unknown'), 'Last known location');
});
