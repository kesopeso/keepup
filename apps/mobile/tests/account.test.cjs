const assert = require('node:assert/strict');
const { test } = require('node:test');
const { createAuthTransport, AccountError } = require('../src/auth/api.ts');
const { createAccountRepository } = require('../src/auth/repository.ts');
const { createAccountController } = require('../src/auth/controller.ts');
const { parseAuthLink, destination, passwordError, usernameError } = require('../src/auth/contracts.ts');
const future = () => new Date(Date.now() + 86400000).toISOString();
const token = (char = 'a') => char.repeat(43);
const user = { id: 'user-1', email: 'person@example.test', username: 'Trail_User', verified: false };
const response = (verified = false, char = 'a') => ({ user: { ...user, verified }, sessionToken: token(char), expiresAt: future() });
function storage() {
  const data = new Map();
  return { data, async getItem(k) { return data.get(k) ?? null; }, async setItem(k, v) { data.set(k, v); }, async deleteItem(k) { data.delete(k); } };
}
function harness(api) { const store = storage(); const repository = createAccountRepository(store, 'api-one'); return { store, repository, controller: createAccountController(api, repository) }; }

test('native transport omits cookies and uses explicit native authority with JSON-only bodies', async () => {
  const seen = [];
  const api = createAuthTransport('https://keepup.test/api', async (url, opts) => { seen.push({ url, ...opts }); return new Response(JSON.stringify({ status: 'ok' })); });
  await api('/auth/login', { identifier: 'User', password: 'Exact !1' });
  await api('/auth/me', undefined, token());
  assert.equal(seen[0].url, 'https://keepup.test/api/auth/login');
  assert.equal(seen[0].credentials, 'omit');
  assert.equal(seen[0].headers['X-KeepUp-Client'], 'native');
  assert.equal(seen[0].headers.Origin, undefined);
  assert.equal(seen[0].headers.Cookie, undefined);
  assert.equal(seen[0].headers['X-KeepUp-Request'], undefined);
  assert.equal(seen[0].body, '{"identifier":"User","password":"Exact !1"}');
  assert.equal(seen[1].headers.Authorization, `Bearer ${token()}`);
});

test('transport preserves rate limits and bounds stalled body reads', async (t) => {
  const limited = createAuthTransport('https://keepup.test/api', async () => new Response('{"error":"rate_limited"}', { status: 429, headers: { 'Retry-After': '60' } }));
  await assert.rejects(limited('/auth/login', {}), (e) => e.code === 'rate_limited' && e.retryAfter === 60);
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const stalled = createAuthTransport('https://keepup.test/api', async (_url, { signal }) => ({ status: 200, ok: true, json: () => new Promise((_r, reject) => signal.addEventListener('abort', () => reject(Error('aborted')))) }));
  const result = stalled('/auth/me');
  await Promise.resolve(); t.mock.timers.tick(15000);
  await assert.rejects(result, (e) => ['invalid_response', 'request_cancelled'].includes(e.code));
});

test('links accept only configured HTTPS hosts, approved paths and fragment tokens', () => {
  const origin = 'https://keepup.test';
  assert.deepEqual(parseAuthLink(`${origin}/auth/verify#token=${token()}`, origin), { screen: 'verify', token: token() });
  assert.deepEqual(parseAuthLink(`${origin}/routes/ABC123`, origin), { screen: 'destination', destination: '/routes/ABC123' });
  for (const value of [`${origin}/auth/verify?token=${token()}`, `https://evil.test/auth/verify#token=${token()}`, `${origin}/auth/verify#token=short`, `${origin}/routes/ABC123#credential=secret`, `${origin}/auth/reset-password`, `${origin}/routes/ABC123?next=evil`]) assert.equal(parseAuthLink(value, origin), null);
  for (const value of ['//evil.test', '/routes/../../account', '/account?next=x', '/routes/ABC123/extra']) assert.equal(destination(value), null);
  assert.equal(usernameError('Trail_User-1'), null);
  assert.equal(passwordError(' 123456😀'), null);
  assert.ok(passwordError('12345678 '));
});

test('secure records isolate backends, expire contexts and never retain passwords or link proofs', async () => {
  const store = storage(); let now = Date.now();
  const one = createAccountRepository(store, 'one', () => now);
  const two = createAccountRepository(store, 'two', () => now);
  await one.saveSession(response()); await one.saveFlow({ flowToken: token('f'), expiresAt: future() }); await one.saveDestination('/routes/ABC123');
  assert.equal(await two.session(), null); assert.equal(await two.destination(), null);
  assert.equal((await one.session()).user.username, user.username);
  assert.equal(await one.destination(), '/routes/ABC123');
  now += 2 * 86400000;
  assert.equal(await one.session(), null); assert.equal(await one.flow(), null); assert.equal(await one.destination(), null);
});

test('pending sessions restore through controller recreation and observe verification elsewhere', async () => {
  let active = false;
  const api = async (path) => path === '/auth/login' ? response() : { user: { ...user, verified: active }, expiresAt: future() };
  const { repository, controller } = harness(api);
  await controller.initialize(); await controller.setDestination('/routes/ABC123'); await controller.login('TRAIL_USER', 'Exact !1');
  const restored = createAccountController(api, repository); await restored.initialize();
  assert.equal(restored.snapshot().user.verified, false); assert.equal(restored.snapshot().destination, '/routes/ABC123');
  active = true; await restored.refresh(); assert.equal(restored.snapshot().user.verified, true);
  assert.equal(JSON.stringify(restored.snapshot()).includes('sessionToken'), false);
});

test('password change serialization saves rotated token before the next authenticated read', async () => {
  const calls = [];
  const { controller } = harness(async (path, body, credential) => {
    calls.push([path, credential]);
    if (path === '/auth/login') return response(true);
    if (path === '/auth/password/change') return response(true, 'b');
    return { user: { ...user, verified: true }, expiresAt: future() };
  });
  await controller.initialize(); await controller.login('Trail_User', 'Old !1');
  await Promise.all([controller.changePassword('Old !1', 'Newpass!1'), controller.refresh()]);
  assert.deepEqual(calls.slice(-2), [['/auth/password/change', token()], ['/auth/me', token('b')]]);
});

test('wrong current password retains session, revocation clears it, temporary failures retain credentials', async () => {
  let failure = null;
  const { controller, repository } = harness(async (path) => {
    if (path === '/auth/login') return response(true);
    if (failure) throw new AccountError(failure, 401);
    return { user: { ...user, verified: true }, expiresAt: future() };
  });
  await controller.initialize(); await controller.login('Trail_User', 'Exact !1');
  failure = 'invalid_credentials'; await assert.rejects(controller.changePassword('wrong', 'Newpass!1'));
  assert.ok(controller.snapshot().user); assert.ok(await repository.session());
  failure = 'offline'; await assert.rejects(controller.refresh()); assert.ok(controller.snapshot().user);
  failure = 'session_expired'; await assert.rejects(controller.refresh()); assert.equal(controller.snapshot().user, null); assert.equal(await repository.session(), null);
});

test('secure-write and Google-context deletion failures can retry the same issued session without another signup', async () => {
  for (const failing of ['setItem', 'deleteItem']) {
    let loginCalls = 0; const store = storage(); const repo = createAccountRepository(store, 'one');
    await repo.saveFlow({ flowToken: token('f'), expiresAt: future() });
    const controller = createAccountController(async (path) => {
      if (path === '/auth/google/context') return { status: 'onboarding', expiresAt: future(), email: user.email };
      loginCalls++; return response(true);
    }, repo);
    await controller.initialize();
    const original = store[failing]; let fail = true;
    store[failing] = async (...args) => { if (fail) throw Error('secure storage unavailable'); return original(...args); };
    await assert.rejects(controller.login('Trail_User', 'Exact !1')); assert.equal(controller.snapshot().saving, true);
    fail = false; await controller.retrySave(); assert.equal(controller.snapshot().saving, false); assert.ok(controller.snapshot().user);
    assert.equal(loginCalls, 1);
  }
});

test('Google flow uses separate credential, restores onboarding and exchanges only through an explicit action', async () => {
  const calls = [];
  const api = async (path, body, credential) => {
    calls.push([path, body, credential]);
    if (path === '/auth/google/start') return { nonce: token('n'), flowToken: token('f'), expiresAt: future() };
    if (path === '/auth/google/proof') return { status: 'email_verification_required', expiresAt: future() };
    if (path === '/auth/google/context') return { status: 'ready', expiresAt: future() };
    if (path === '/auth/google/exchange') return { status: 'onboarding', email: user.email, username: user.username, expiresAt: future() };
    if (path === '/auth/google/complete') return { status: 'authenticated', ...response(true) };
  };
  const { controller, repository } = harness(api); await controller.initialize(); await controller.setDestination('/routes/ABC123');
  await controller.startGoogle(async (nonce) => { assert.equal(nonce, token('n')); return 'signed-proof'; });
  assert.equal(controller.snapshot().google.status, 'email_verification_required');
  const restored = createAccountController(api, repository); await restored.initialize();
  assert.equal(restored.snapshot().google.status, 'ready'); assert.equal(restored.snapshot().user, null);
  await restored.exchangeGoogle(); assert.equal(restored.snapshot().google.username, user.username);
  await restored.completeGoogle(user.username, 'Setup123!'); assert.ok(restored.snapshot().user.verified);
  assert.equal(await repository.flow(), null); assert.equal(restored.snapshot().destination, '/routes/ABC123');
  for (const [path, , credential] of calls.filter(([path]) => !path.endsWith('/start'))) assert.equal(credential, token('f'), path);
});

test('Google cancellation and late picker result cannot submit a proof or resurrect a cancelled flow', async () => {
  let resolvePicker; const calls = [];
  const { controller, repository } = harness(async (path) => { calls.push(path); return { nonce: token('n'), flowToken: token('f'), expiresAt: future() }; });
  await controller.initialize();
  const pickerReady = Promise.withResolvers();
  const signing = controller.startGoogle(() => { pickerReady.resolve(); return new Promise((resolve) => { resolvePicker = resolve; }); });
  await pickerReady.promise; await controller.cancelGoogle(); resolvePicker('late-proof'); await signing;
  assert.deepEqual(calls, ['/auth/google/start']); assert.equal(await repository.flow(), null);
  await assert.rejects(controller.startGoogle(async () => null), (e) => e.code === 'cancelled');
});

test('link confirmation never signs in and reset clears access without activation', async () => {
  const { controller } = harness(async (path) => path === '/auth/login' ? response() : undefined);
  await controller.initialize(); await controller.confirm('verify', token('v')); assert.equal(controller.snapshot().user, null);
  await controller.login('Trail_User', 'Exact !1'); await controller.reset(token('r'), 'Reset123!'); assert.equal(controller.snapshot().user, null);
});

test('logout waits for in-flight rotation, clears destination and cannot revive after restoration', async () => {
  const { controller, repository } = harness(async (path) => path === '/auth/login' ? response(true) : path === '/auth/password/change' ? response(true, 'b') : undefined);
  await controller.initialize(); await controller.login('Trail_User', 'Exact !1'); await controller.setDestination('/routes/ABC123');
  await Promise.all([controller.changePassword('Exact !1', 'Newpass!1'), controller.logout(true)]);
  assert.equal(controller.snapshot().user, null); assert.equal(await repository.session(), null); assert.equal(await repository.destination(), null);
});

test('retained destination is rechecked and unavailable route falls back without granting access', async () => {
  const { controller } = harness(async (path) => {
    if (path === '/auth/login') return response(true);
    if (path.endsWith('/access')) throw new AccountError('route_not_found', 404);
  });
  await controller.initialize(); await controller.setDestination('/routes/ABC123'); await controller.login('Trail_User', 'Exact !1');
  assert.equal(await controller.checkDestination(), '/my-routes'); assert.equal(controller.snapshot().destination, '/my-routes');
});

test('secure-store read failure can retry initialization without losing the saved session', async () => {
  const store = storage(); const repo = createAccountRepository(store, 'one');
  await repo.saveSession(response(true));
  const original = store.getItem; let fail = true;
  store.getItem = async (...args) => { if (fail) throw Error('locked keystore'); return original(...args); };
  const controller = createAccountController(async () => ({ user: { ...user, verified: true }, expiresAt: future() }), repo);
  await assert.rejects(controller.initialize()); assert.equal(controller.snapshot().ready, false);
  fail = false; await controller.initialize(); assert.equal(controller.snapshot().ready, true); assert.equal(controller.snapshot().user.username, user.username);
});

test('polling cannot discard an unsaved rotated credential by probing its revoked predecessor', async () => {
  let reads = 0;
  const { store, controller, repository } = harness(async (path) => {
    if (path === '/auth/login') return response(true);
    if (path === '/auth/password/change') return response(true, 'b');
    reads++; throw new AccountError('session_expired', 401);
  });
  await controller.initialize(); await controller.login('Trail_User', 'Exact !1');
  const write = store.setItem; let fail = true;
  store.setItem = async (...args) => { if (fail) throw Error('keystore unavailable'); return write(...args); };
  await assert.rejects(controller.changePassword('Exact !1', 'Newpass123!'));
  await controller.refresh(); assert.equal(reads, 0); assert.equal(controller.snapshot().saving, true);
  fail = false; await controller.retrySave(); assert.equal((await repository.session()).sessionToken, token('b'));
});

test('routine authority refresh never enters blocking credential-save state or unmounts forms', async () => {
  let verified = false;
  const { controller } = harness(async path => path === '/auth/login' ? response() : {user:{...user,verified},expiresAt:future()});
  await controller.initialize(); await controller.login('Trail_User','Exact !1');
  const saving = []; controller.subscribe(() => saving.push(controller.snapshot().saving));
  verified = true; await controller.refresh();
  assert.deepEqual(saving,[false]); assert.equal(controller.snapshot().user.verified,true);
});
