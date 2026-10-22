const { test } = require('node:test');
const assert = require('node:assert/strict');
const { createAuthTransport, AccountError } = require('../src/auth/api.ts');
const { createAccountController } = require('../src/auth/controller.ts');
const { createAccountRepository } = require('../src/auth/repository.ts');
const { createAccountRoutesApi, listQuery, appendPage, freshList, defaultFilters } = require('../src/auth/route-api.ts');
const { routeShareUrl, shareRouteLink } = require('../src/domain/route-share.ts');
const token = 'a'.repeat(43), rotated = 'b'.repeat(43);
const session = (sessionToken = token) => ({ user: { id: 'one', username: 'Trail_User', email: 'one@example.test', verified: true }, sessionToken, expiresAt: new Date(Date.now() + 86400000).toISOString() });
function harness(api) {
  const data = new Map();
  const repository = createAccountRepository({ getItem: async k => data.get(k) ?? null, setItem: async (k,v) => data.set(k,v), deleteItem: async k => data.delete(k) }, 'test');
  return { controller: createAccountController(api, repository), data };
}
test('route requests serialize after password rotation and never send device profile authority', async () => {
  const seen = [];
  const h = harness(async (path, body, credential, signal, method) => {
    seen.push({ path, body, credential, method });
    if (path === '/auth/login') return session();
    if (path === '/auth/password/change') return session(rotated);
    if (path === '/routes') return { route: { code: 'ABC123' } };
    return {};
  });
  await h.controller.login('Trail_User', 'secret'); const api = createAccountRoutesApi(h.controller);
  const change = h.controller.changePassword('old', 'new');
  await api.create({ name: 'Convoy', description: '', password: '', sharingPolicy: 'everyone_can_share', transportMode: 'car' }, new AbortController().signal); await change;
  const call = seen.at(-1); assert.equal(call.credential, rotated);
  assert.equal(call.body.clientId, undefined); assert.equal(call.body.displayName, undefined);
  await api.leave('ABC123'); assert.equal(seen.at(-1).method, 'DELETE');
  await api.close({ route: { code: 'ABC123', name: 'Convoy', description: '' } }); assert.equal(seen.at(-1).method, 'PATCH');
  await api.remove('ABC123'); assert.equal(seen.at(-1).method, 'DELETE');
});
test('route session_expired clears authority but a wrong route password preserves account access', async () => {
  let code = 'invalid_password';
  const h = harness(async path => { if (path === '/auth/login') return session(); throw new AccountError(code, 401); });
  await h.controller.login('one', 'secret'); const api = createAccountRoutesApi(h.controller);
  await assert.rejects(api.join('ABC123', { password: 'wrong', transportMode: 'car' }, new AbortController().signal));
  assert.equal(h.controller.snapshot().user.id, 'one'); code = 'session_expired';
  await assert.rejects(api.join('ABC123', {}, new AbortController().signal));
  assert.equal(h.controller.snapshot().user, null); assert.equal(h.data.size, 0);
});
test('native PATCH and DELETE transport omits cookie authority', async () => {
  const seen = [];
  const transport = createAuthTransport('https://keepup.test/api', async (url, options) => { seen.push(options); return new Response(null, { status: 204 }); });
  await transport('/routes/ABC123', {}, token, undefined, 'DELETE');
  assert.equal(seen[0].method, 'DELETE'); assert.equal(seen[0].credentials, 'omit'); assert.equal(seen[0].headers['X-KeepUp-Client'], 'native');
});
test('all category combinations and tabs bind pagination; merged pages deduplicate route ids', () => {
  for (const view of ['active', 'archived']) for (let mask = 0; mask < 8; mask++) {
    const filters = { view, owned: !!(mask & 1), joined: !!(mask & 2), left: !!(mask & 4) };
    const q = new URLSearchParams(listQuery(filters, 'opaque/+cursor'));
    for (const key of ['owned', 'joined', 'left']) assert.equal(q.get(key), String(filters[key]));
    assert.equal(q.get('view'), view); assert.equal(q.get('cursor'), 'opaque/+cursor'); assert.equal(q.get('limit'), '20');
  }
  assert.deepEqual(freshList().filters, defaultFilters); assert.equal(defaultFilters.left, false);
  const result = appendPage({ items: [{ id: 'one' }, { id: 'two' }], counts: { active: 2, archived: 0 }, nextCursor: 'a' }, { items: [{ id: 'two', name: 'updated' }, { id: 'three' }], counts: { active: 3, archived: 1 }, nextCursor: null });
  assert.deepEqual(result.items.map(i => i.id), ['one', 'two', 'three']); assert.equal(result.items[1].name, 'updated'); assert.equal(result.counts.active, 3);
});
test('Share uses credential-free public links for active, archived, left and cached rows; cancellation is silent', async () => {
  for (const _state of ['active', 'archived', 'left', 'cached']) {
    const url = routeShareUrl('ABC123', 'https://api.test/prefix/api', 'https://keepup.test');
    assert.equal(url, 'https://keepup.test/routes/ABC123');
    assert.equal(await shareRouteLink(url, async () => ({ action: 'dismissedAction' })), null);
    assert.match(await shareRouteLink(url, async () => { throw Error(); }), /Copy this link: https:\/\/keepup.test\/routes\/ABC123/);
  }
  assert.equal(routeShareUrl('ABC123', 'https://keepup.test/prefix/api'), 'https://keepup.test/prefix/routes/ABC123');
  assert.throws(() => routeShareUrl('bad', 'https://keepup.test/api'));
});
test('Android account socket suppresses React Native automatic browser Origin', () => {
  const { nativeAccountSocket } = require('../src/live/native-account-socket.ts');
  let seen;
  nativeAccountSocket('wss://keepup.test/ws', class { constructor(...args) { seen = args; } });
  assert.deepEqual(seen, ['wss://keepup.test/ws', undefined, { headers: { 'X-KeepUp-Client': 'native', Origin: '' } }]);
  assert.equal(seen[0].includes('?'), false);
});
test('native fetch and body implementations that ignore abort cannot hold the account queue', async t => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const body = createAuthTransport('https://keepup.test', async () => ({ status: 200, ok: true, json: () => new Promise(() => {}) }));
  const read = body('/healthz'); await Promise.resolve(); await Promise.resolve(); t.mock.timers.tick(15000);
  await assert.rejects(read, e => e.code === 'request_cancelled');
  const external = new AbortController(); let first = true;
  const transport = createAuthTransport('https://keepup.test', async () => {
    if (first) { first = false; return new Promise(() => {}); }
    return new Response(JSON.stringify(session()));
  });
  const cancelled = transport('/healthz', undefined, undefined, external.signal); external.abort();
  await assert.rejects(cancelled, e => e.code === 'request_cancelled');
  assert.equal((await transport('/auth/login', {})).user.id, 'one');
});
