const assert = require('node:assert/strict');
const { test } = require('node:test');
const { getApiBaseUrl } = require('../src/api/config.ts');
const { checkApiHealth, HEALTH_CHECK_TIMEOUT_MS } = require('../src/api/health.ts');

const baseUrl = 'https://keepup.example/api';
const check = () => checkApiHealth(baseUrl, new AbortController().signal);

test('development uses the Android host, while release requires configuration', () => {
  assert.equal(getApiBaseUrl('', true), 'http://10.0.2.2:3000/api');
  assert.throws(() => getApiBaseUrl('', false), /required/);
  assert.equal(getApiBaseUrl(' https://keepup.example/api/// ', false), baseUrl);
  for (const value of ['not a URL', 'ftp://example.com/api', 'https://user:secret@example.com', 'https://example.com/api?key=value', 'https://example.com/api#fragment']) {
    assert.throws(() => getApiBaseUrl(value, true));
  }
});

test('a fresh healthy response connects through the API prefix', async (t) => {
  t.mock.method(global, 'fetch', async (url, options) => {
    assert.equal(url, `${baseUrl}/healthz`);
    assert.equal(options.cache, 'no-store');
    return new Response(JSON.stringify({ status: 'ok' }));
  });
  assert.deepEqual(await check(), { status: 'connected' });
});

test('a degraded backend and proxy errors remain unavailable', async (t) => {
  for (const status of [503, 502, 404]) {
    t.mock.method(global, 'fetch', async () => new Response('{"status":"degraded"}', { status }));
    assert.deepEqual(await check(), { status: 'unavailable', reason: 'server' });
  }
});

test('HTML, malformed JSON, and unrelated successful responses cannot appear connected', async (t) => {
  for (const body of ['<html>Wrong server</html>', '{', 'null', '{}', '{"status":"degraded"}']) {
    t.mock.method(global, 'fetch', async () => new Response(body));
    assert.deepEqual(await check(), { status: 'unavailable', reason: 'invalid_response' });
  }
});

test('a failed network request can recover on the next check', async (t) => {
  let attempts = 0;
  t.mock.method(global, 'fetch', async () => {
    if (++attempts === 1) throw new TypeError('Network request failed');
    return new Response('{"status":"ok"}');
  });
  assert.deepEqual(await check(), { status: 'unavailable', reason: 'network' });
  assert.deepEqual(await check(), { status: 'connected' });
});

test('a stalled request times out and aborts the transport', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let requestSignal;
  t.mock.method(global, 'fetch', (_url, { signal }) => new Promise((_resolve, reject) => {
    requestSignal = signal;
    signal.addEventListener('abort', () => reject(new Error('Aborted')), { once: true });
  }));
  const result = check();
  t.mock.timers.tick(HEALTH_CHECK_TIMEOUT_MS);
  assert.deepEqual(await result, { status: 'unavailable', reason: 'timeout' });
  assert.equal(requestSignal.aborted, true);
});

test('the timeout also covers a response body that never finishes', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  t.mock.method(global, 'fetch', async (_url, { signal }) => ({
    ok: true,
    json: () => new Promise((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new Error('Aborted')), { once: true });
    }),
  }));
  const result = check();
  await Promise.resolve();
  t.mock.timers.tick(HEALTH_CHECK_TIMEOUT_MS);
  assert.deepEqual(await result, { status: 'unavailable', reason: 'timeout' });
});

test('screen cancellation aborts the request without reporting unavailability', async (t) => {
  const controller = new AbortController();
  let requestSignal;
  t.mock.method(global, 'fetch', (_url, { signal }) => new Promise((_resolve, reject) => {
    requestSignal = signal;
    signal.addEventListener('abort', () => reject(new Error('Screen left')), { once: true });
  }));
  const result = checkApiHealth(baseUrl, controller.signal);
  controller.abort();
  await assert.rejects(result, /Screen left/);
  assert.equal(requestSignal.aborted, true);
});
