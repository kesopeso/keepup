import assert from "node:assert/strict";
import test from "node:test";
import { authRequest, accountRequest, AuthError, passwordError, usernameError } from "./auth-api.ts";
import { checkedDestination, rememberDestination, safeDestination, savedDestination } from "./auth-destination.ts";

test("signup password validation counts Unicode characters without trimming or changing passwords", () => {
  for (const value of ["Trail12!", "  Trail1!  ", "🧭🧭🧭🧭🧭🧭🧭1", "密碼密碼密碼1。"]) assert.equal(passwordError(value), undefined, value);
  for (const value of ["short1!", "password1 ", "password!", "a".repeat(127) + "1!", "🧭🧭🧭1"]) assert.ok(passwordError(value), value);
  assert.equal(usernameError("Trail-Rider_9"), undefined);
  for (const value of [" abcd", "ab", "råider", "a".repeat(25)]) assert.ok(usernameError(value));
});

test("only exact approved destinations survive storage and reload", (t) => {
  const values = new Map<string, string>();
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
  Object.defineProperty(globalThis, "localStorage", { configurable: true, value: {
    setItem: (key: string, value: string) => values.set(key, value), getItem: (key: string) => values.get(key) ?? null, removeItem: (key: string) => values.delete(key),
  } });
  t.after(() => { if (descriptor) Object.defineProperty(globalThis, "localStorage", descriptor); else Reflect.deleteProperty(globalThis, "localStorage"); });
  for (const value of ["https://evil.test", "//evil.test", "/\\evil", "/routes/ABC123?next=//evil", "/routes/ABC123#token=secret", "/routes/%2f%2fevil", "/auth/login", "/account/../evil", "/routes/ABC123/"]) assert.equal(safeDestination(value), null);
  rememberDestination("/routes/abc123"); assert.equal(savedDestination(), "/routes/ABC123");
  rememberDestination("//evil.test"); assert.equal(savedDestination(), "/my-routes");
  rememberDestination("/account");
  t.mock.method(Date, "now", () => Number.MAX_SAFE_INTEGER);
  assert.equal(savedDestination(), "/my-routes");
});

test("browser auth requests use same-origin cookies and CSRF headers, without native credentials", async (t) => {
  const calls: { url: string; init?: RequestInit }[] = [];
  t.mock.method(globalThis, "fetch", async (url: string, init?: RequestInit) => {
    calls.push({ url, init }); return new Response(JSON.stringify({ status: "accepted" }), { status: 202 });
  });
  await authRequest("signup", { email: "test@example.com", username: "Rider", password: "Trail12!" });
  assert.equal(calls[0].url, "/api/auth/signup");
  assert.equal(calls[0].init?.credentials, "same-origin");
  assert.equal(calls[0].init?.cache, "no-store");
  assert.deepEqual(calls[0].init?.headers, { "Content-Type": "application/json", "X-KeepUp-Request": "1" });
  assert.equal(JSON.parse(String(calls[0].init?.body)).password, "Trail12!");
  await authRequest("me");
  assert.equal(calls[1].init?.method, "GET");
  assert.equal(calls[1].init?.body, undefined);
});

test("requests serialize cookie rotation with reads and recover after failures", async (t) => {
  const calls: string[] = [];
  let release!: () => void;
  const first = new Promise<void>((resolve) => { release = resolve; });
  t.mock.method(globalThis, "fetch", async (url: string) => {
    calls.push(url);
    if (url.endsWith("change")) { await first; return new Response(null, { status: 204 }); }
    return new Response(JSON.stringify({ error: "rate_limited" }), { status: 429, headers: { "Retry-After": "61" } });
  });
  const mutation = authRequest("password/change", {});
  const read = accountRequest("/me/routes?view=active");
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(calls, ["/api/auth/password/change"]);
  release(); await mutation;
  await assert.rejects(read, (error: unknown) => error instanceof AuthError && error.code === "rate_limited" && error.retryAfter === 61);
  await assert.rejects(authRequest("me"), AuthError);
  assert.equal(calls.length, 3);
});

test("destination checks preserve invitations, reject deleted/forbidden access and propagate expiry", async (t) => {
  let status = 200;
  t.mock.method(globalThis, "fetch", async () => new Response(null, { status }));
  assert.deepEqual(await checkedDestination("/routes/ABC123"), { path: "/routes/ABC123", unavailable: false });
  for (status of [403, 404]) assert.deepEqual(await checkedDestination("/routes/ABC123"), { path: "/my-routes", unavailable: true });
  status = 401; await assert.rejects(checkedDestination("/routes/ABC123"), /session_expired/);
  status = 503; await assert.rejects(checkedDestination("/routes/ABC123"), /destination_unavailable/);
});
