const assert = require('node:assert/strict');
const { test } = require('node:test');
const { createRoutesApi, RouteApiError } = require('../src/api/routes.ts');
const { createRouteSession, normalizeCreateRequest } = require('../src/domain/create-session.ts');
const { loadMemberSnapshot } = require('../src/domain/join-session.ts');
const { createSessionRepository, StorageError } = require('../src/storage/session-repository.ts');

const input = { clientId: 'device-id', displayName: ' Ana ', transportMode: 'bicycle', name: ' Morning convoy ', description: ' Meet at the station ', password: '  secret  ', sharingPolicy: 'joiners_can_view_only' };
const route = { code: 'NEW123', name: 'Morning convoy', description: 'Meet at the station', status: 'active', sharingPolicy: input.sharingPolicy };
const result = { route, owner: { id: 'owner-id' }, memberToken: 'member-secret', ownerToken: 'owner-secret' };
const member = { code: route.code, memberId: result.owner.id, memberToken: result.memberToken, ownerToken: result.ownerToken };
const signal = () => new AbortController().signal;
function storage() {
  const values = new Map();
  return { values, getItem: async key => values.get(key) ?? null, setItem: async (key, value) => { values.set(key, value); }, deleteItem: async key => { values.delete(key); } };
}
const repository = store => createSessionRepository(store, 'test-backend', () => input.clientId);

test('creation validates required fields and normalizes names without changing passwords', () => {
  const normalized = normalizeCreateRequest(input);
  assert.equal(normalized.name, 'Morning convoy');
  assert.equal(normalized.displayName, 'Ana');
  assert.equal(normalized.description, 'Meet at the station');
  assert.equal(normalized.password, input.password);
  for (const [key, value] of [['name', '  '], ['displayName', ''], ['clientId', ''], ['transportMode', 'rocket'], ['sharingPolicy', 'invalid']]) {
    assert.throws(() => normalizeCreateRequest({ ...input, [key]: value }));
  }
});

test('creation posts the shared contract and rejects missing owner access', async t => {
  const client = createRoutesApi('https://keepup.example/api');
  let request;
  t.mock.method(global, 'fetch', async (url, options) => { request = { url, options }; return Response.json(result, { status: 201 }); });
  assert.deepEqual(await client.create(input, signal()), result);
  assert.equal(request.url, 'https://keepup.example/api/routes');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.headers['Content-Type'], 'application/json');
  assert.deepEqual(JSON.parse(request.options.body), input);
  for (const invalid of [{ ...result, ownerToken: '' }, { ...result, memberToken: '' }, { ...result, owner: {} }, { ...result, route: { ...route, status: 'closed' } }, { ...result, route: { ...route, code: 'bad' } }]) {
    t.mock.method(global, 'fetch', async () => Response.json(invalid));
    await assert.rejects(client.create(input, signal()), error => error.code === 'invalid_response');
  }
});

test('new owner and member access survives restart, scoped to its backend, without storing the password', async () => {
  const store = storage();
  const repo = repository(store);
  const session = createRouteSession({ create: async () => result }, repo);
  const created = await session.submit(input, signal());
  assert.deepEqual(created.member, member);
  assert.deepEqual(await repository(store).getMembership(route.code), member);
  assert.deepEqual(await repository(store).getProfile(), { clientId: input.clientId, displayName: 'Ana', transportMode: 'bicycle' });
  assert.equal(await repository(store).getLastCode(), route.code);
  assert.equal(await createSessionRepository(store, 'other-backend', () => 'other-device').getMembership(route.code), null);
  assert.ok([...store.values.values()].every(value => !value.includes(input.password)));
  assert.equal((await loadMemberSnapshot({ getSnapshot: async () => ({ route, members: [], viewer: { memberId: member.memberId, role: 'owner' } }) }, repo, member, signal())).viewer.role, 'owner');
});

for (const failedKey of ['.route.', '.lastRoute']) {
  test(`retry after ${failedKey} storage failure saves the original tokens without creating twice`, async () => {
    const store = storage();
    const write = store.setItem;
    let fail = true;
    store.setItem = async (key, value) => { if (fail && key.includes(failedKey)) throw Error('Keystore unavailable'); await write(key, value); };
    let creates = 0;
    const repo = repository(store);
    const session = createRouteSession({ create: async () => { creates++; return result; } }, repo);
    await assert.rejects(session.submit(input, signal()), StorageError);
    assert.equal(session.getCreatedCode(), route.code);
    fail = false;
    assert.deepEqual((await session.submit({ ...input, name: 'Changed form' }, signal())).member, member);
    assert.equal(creates, 1);
    assert.deepEqual(await repository(store).getMembership(route.code), member);
    assert.equal(await repository(store).getLastCode(), route.code);
  });
}

test('rapid concurrent submissions share one create request', async () => {
  let finish;
  let creates = 0;
  const session = createRouteSession({ create: async () => { creates++; return await new Promise(resolve => { finish = resolve; }); } }, repository(storage()));
  const first = session.submit(input, signal());
  const second = session.submit(input, signal());
  assert.equal(first, second);
  await new Promise(resolve => setImmediate(resolve));
  finish(result);
  await Promise.all([first, second]);
  assert.equal(creates, 1);
});

test('validation or profile-storage failure never sends a create request', async () => {
  let creates = 0;
  const api = { create: async () => { creates++; return result; } };
  await assert.rejects(createRouteSession(api, repository(storage())).submit({ ...input, name: '  ' }, signal()), /route name/);
  const store = storage();
  store.setItem = async () => { throw Error('Unavailable'); };
  await assert.rejects(createRouteSession(api, repository(store)).submit(input, signal()), StorageError);
  assert.equal(creates, 0);
});

test('a rejected request retains form retry without saving credentials', async () => {
  const store = storage();
  let creates = 0;
  const session = createRouteSession({ create: async () => { if (++creates === 1) throw new RouteApiError('Unavailable', 503); return result; } }, repository(store));
  await assert.rejects(session.submit(input, signal()), RouteApiError);
  assert.equal(session.getCreatedCode(), null);
  assert.equal(await repository(store).getMembership(route.code), null);
  await session.submit(input, signal());
  assert.equal(creates, 2);
});

test('snapshot failure retries with saved owner membership rather than creating another route', async () => {
  const store = storage();
  const repo = repository(store);
  let creates = 0;
  const api = { create: async () => { creates++; return result; }, getSnapshot: async () => { throw new RouteApiError('Unavailable', 503); } };
  const session = createRouteSession(api, repo);
  await session.submit(input, signal());
  await assert.rejects(loadMemberSnapshot(api, repo, member, signal()));
  assert.deepEqual(await repository(store).getMembership(route.code), member);
  await session.submit(input, signal());
  assert.equal(creates, 1);
});

test('cancellation during profile saving prevents creation', async () => {
  const controller = new AbortController();
  let creates = 0;
  const session = createRouteSession({ create: async () => { creates++; return result; } }, { saveProfile: async () => controller.abort() });
  await assert.rejects(session.submit(input, controller.signal));
  assert.equal(creates, 0);
});

test('invalid owner membership removes both credentials and last-route restoration', async () => {
  const store = storage();
  const repo = repository(store);
  await repo.saveMembership(member);
  await assert.rejects(loadMemberSnapshot({ getSnapshot: async () => { throw new RouteApiError('Invalid access', 401, 'unauthorized'); } }, repo, member, signal()));
  assert.equal(await repository(store).getMembership(route.code), null);
  assert.equal(await repository(store).getLastCode(), null);
});
