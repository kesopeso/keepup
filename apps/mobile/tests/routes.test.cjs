const assert = require('node:assert/strict');
const { test } = require('node:test');
const { parseRouteCode } = require('../src/domain/routes.ts');
const { createRoutesApi, RouteApiError, ROUTE_REQUEST_TIMEOUT_MS } = require('../src/api/routes.ts');
const { createSessionRepository, StorageError } = require('../src/storage/session-repository.ts');
const { joinOrResume, loadMemberSnapshot } = require('../src/domain/join-session.ts');

const route = { id: 'route-id', code: 'ABC123', name: 'Test route', description: '', status: 'active', sharingPolicy: 'everyone_can_share', hasPassword: false, maxTrackingMembers: 10, createdAt: '2026-10-05T08:00:00Z', closedAt: null };
const member = { code: route.code, memberId: 'member-id', memberToken: 'private-token' };
const profile = { clientId: 'device-id', displayName: 'Android', transportMode: 'car' };
const input = { ...profile, password: 'private-password' };
const snapshot = { route, members: [], viewer: { memberId: member.memberId } };
const signal = () => new AbortController().signal;
const api = () => createRoutesApi('https://keepup.example/api');

function storage() {
  const values = new Map();
  return {
    values,
    getItem: async (key) => values.get(key) ?? null,
    setItem: async (key, value) => { values.set(key, value); },
    deleteItem: async (key) => { values.delete(key); },
  };
}
function repository(store = storage(), namespace = 'backend-one') {
  return createSessionRepository(store, namespace, () => profile.clientId);
}

test('route codes and pasted links normalize without using the link host for API requests', () => {
  for (const value of [' abc123 ', 'https://keepup.example/routes/abc123', 'http://localhost:3000/routes/ABC123/?source=share#route']) {
    assert.equal(parseRouteCode(value), route.code);
  }
  for (const value of ['', 'ABC', 'ABC1234', 'https://example.com/other/ABC123', 'file:///routes/ABC123', 'https://example.com/routes/ABC123/extra']) {
    assert.throws(() => parseRouteCode(value), /six-character/);
  }
});

test('access metadata precedes joining, and a snapshot uses Bearer authentication', async (t) => {
  const calls = [];
  t.mock.method(global, 'fetch', async (url, options) => {
    calls.push({ url, options });
    if (url.endsWith('/access')) return Response.json({ ...route, requiresPassword: true });
    if (url.endsWith('/members')) return Response.json({ route, member: { id: member.memberId }, memberToken: member.memberToken });
    return Response.json(snapshot);
  });
  const client = api();
  assert.equal((await client.getAccess(route.code, signal())).requiresPassword, true);
  await client.join(route.code, input, signal());
  await client.getSnapshot(route.code, member.memberToken, signal());
  assert.equal(calls[1].options.method, 'POST');
  assert.deepEqual(JSON.parse(calls[1].options.body), input);
  assert.equal(calls[2].options.headers.Authorization, `Bearer ${member.memberToken}`);
  assert.equal(calls[2].options.cache, 'no-store');
});

test('API error codes distinguish incorrect passwords, duplicate names, and closed routes', async (t) => {
  for (const [status, code, message] of [[401, 'invalid_password', /password is not correct/], [409, 'alias_taken', /name is already used/], [409, 'route_closed', /closed/], [404, 'route_not_found', /not found/], [503, 'internal_error', /temporarily unavailable/]]) {
    t.mock.method(global, 'fetch', async () => Response.json({ error: code }, { status }));
    await assert.rejects(api().join(route.code, input, signal()), (error) => error instanceof RouteApiError && error.code === code && message.test(error.message));
  }
});

test('malformed snapshots and responses for another route cannot be displayed', async (t) => {
  for (const body of ['<html>Wrong server</html>', JSON.stringify({ ...snapshot, route: { ...route, code: 'OTHER1' } }), JSON.stringify({ ...snapshot, members: [{}] })]) {
    t.mock.method(global, 'fetch', async () => new Response(body));
    await assert.rejects(api().getSnapshot(route.code, member.memberToken, signal()), (error) => error.code === 'invalid_response');
  }
});

test('route requests abort on timeout', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  t.mock.method(global, 'fetch', (_url, { signal }) => new Promise((_resolve, reject) => {
    signal.addEventListener('abort', () => reject(new Error('Aborted')), { once: true });
  }));
  const result = api().getAccess(route.code, signal());
  t.mock.timers.tick(ROUTE_REQUEST_TIMEOUT_MS);
  await assert.rejects(result, (error) => error.code === 'timeout');
});

test('device identity and credentials survive repository recreation and stay scoped to the backend', async () => {
  const store = storage();
  const first = repository(store);
  assert.equal((await first.getProfile()).clientId, profile.clientId);
  await first.saveProfile(profile);
  await first.saveMembership(member);
  const reopened = repository(store);
  assert.deepEqual(await reopened.getProfile(), profile);
  assert.deepEqual(await reopened.getMembership(route.code), member);
  assert.equal(await reopened.getLastCode(), route.code);
  assert.equal(await repository(store, 'backend-two').getMembership(route.code), null);
  assert.equal(await repository(store, 'backend-two').getLastCode(), null);
  assert.ok([...store.values.values()].every((value) => !value.includes(input.password)));
});

test('saved membership bypasses POST and password entry when reopening a route', async () => {
  const store = storage();
  await repository(store).saveMembership(member);
  const saved = await joinOrResume({ join: async () => { throw new Error('Must not POST'); } }, repository(store), route.code, input, signal());
  assert.deepEqual(saved, member);
});

test('snapshot failure preserves successful join credentials for retry without duplicate membership', async () => {
  const store = storage();
  let joins = 0;
  const client = {
    join: async () => { joins++; return { route, member: { id: member.memberId }, memberToken: member.memberToken }; },
    getSnapshot: async () => { throw new RouteApiError('Unavailable', 503, 'internal_error'); },
  };
  const repo = repository(store);
  await joinOrResume(client, repo, route.code, input, signal());
  await assert.rejects(loadMemberSnapshot(client, repo, member, signal()));
  await joinOrResume(client, repository(store), route.code, input, signal());
  assert.equal(joins, 1);
  assert.deepEqual(await repository(store).getMembership(route.code), member);
});

test('a failed credential write retries saving the returned token instead of joining twice', async () => {
  const store = storage();
  const write = store.setItem;
  let fail = true;
  store.setItem = async (key, value) => {
    if (key.includes('.route.') && fail) throw new Error('Keystore unavailable');
    return write(key, value);
  };
  let joins = 0;
  const client = { join: async () => { joins++; return { route, member: { id: member.memberId }, memberToken: member.memberToken }; } };
  const repo = repository(store);
  await assert.rejects(joinOrResume(client, repo, route.code, input, signal()), StorageError);
  fail = false;
  assert.deepEqual(await joinOrResume(client, repo, route.code, input, signal()), member);
  assert.equal(joins, 1);
  assert.deepEqual(await repository(store).getMembership(route.code), member);
});

test('failed profile storage or a rejected join never leaves saved member credentials', async () => {
  const store = storage();
  store.setItem = async () => { throw new Error('Device storage unavailable'); };
  await assert.rejects(joinOrResume({ join: async () => { throw new Error('Must not POST'); } }, repository(store), route.code, input, signal()), StorageError);
  const repo = repository();
  await assert.rejects(joinOrResume({ join: async () => { throw new RouteApiError('Incorrect password', 401, 'invalid_password'); } }, repo, route.code, input, signal()));
  assert.equal(await repo.getMembership(route.code), null);
});

test('invalid or deleted memberships clear saved tokens, while unrelated failures retain them', async () => {
  for (const status of [401, 403, 404, 503]) {
    const repo = repository();
    await repo.saveMembership(member);
    await assert.rejects(loadMemberSnapshot({ getSnapshot: async () => { throw new RouteApiError('Failed', status, status === 404 ? 'route_not_found' : 'unauthorized'); } }, repo, member, signal()));
    assert.deepEqual(await repo.getMembership(route.code), status === 503 ? member : null);
    assert.equal(await repo.getLastCode(), status === 503 ? route.code : null);
  }
});

test('authenticated snapshots must identify the saved member', async () => {
  const repo = repository();
  await repo.saveMembership(member);
  await assert.rejects(loadMemberSnapshot({ getSnapshot: async () => ({ ...snapshot, viewer: { memberId: 'someone-else' } }) }, repo, member, signal()), /different member/);
  assert.deepEqual(await repo.getMembership(route.code), member);
});
