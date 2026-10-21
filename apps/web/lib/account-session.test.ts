import assert from "node:assert/strict";
import test from "node:test";
import { currentSession, refreshAccountSession, setAccountSession, watchAccountSession } from "./account-session.ts";
import { defaultRouteFilters, saveRouteListState, readRouteListState, markRouteListReturn, isRouteListReturn } from "./my-routes-state.ts";

const account = { user: { id: "account-1", email: "rider@example.test", username: "Rider", verified: true }, expiresAt: "2026-11-21T00:00:00Z" };

test("logout and account switch clear retained summaries and return navigation", () => {
  const values = new Map<string, string>();
  Object.defineProperty(globalThis, "sessionStorage", { configurable: true, value: { getItem: (key: string) => values.get(key), setItem: (key: string, value: string) => values.set(key, value), removeItem: (key: string) => values.delete(key) } });
  setAccountSession(account, false);
  saveRouteListState({ userId: account.user.id, filters: defaultRouteFilters, result: null, scroll: 500 });
  markRouteListReturn();
  setAccountSession({ ...account, user: { ...account.user, id: "another-account" } }, false);
  assert.equal(readRouteListState(account.user.id), null);
  assert.equal(isRouteListReturn(), false);
  saveRouteListState({ userId: "another-account", filters: defaultRouteFilters, result: null, scroll: 800 });
  setAccountSession(null, false);
  assert.equal(values.size, 0);
});

test("a delayed session lookup cannot restore an account after local logout", async (t) => {
  let resolve!: (response: Response) => void;
  const response = new Promise<Response>((done) => { resolve = done; });
  t.mock.method(globalThis, "fetch", () => response);
  setAccountSession(account, false);
  const lookup = refreshAccountSession();
  await new Promise((done) => setTimeout(done, 0));
  setAccountSession(null, false);
  resolve(Response.json(account));
  await lookup;
  assert.equal(currentSession(), null);
});

test("cross-tab revocation rechecks authority even while the receiving tab is hidden", async (t) => {
  const windowDescriptor = Object.getOwnPropertyDescriptor(globalThis, "window");
  const documentDescriptor = Object.getOwnPropertyDescriptor(globalThis, "document");
  const browser = Object.assign(new EventTarget(), { setInterval: () => 0 });
  const document = Object.assign(new EventTarget(), { visibilityState: "hidden" });
  Object.defineProperty(globalThis, "window", { configurable: true, value: browser });
  Object.defineProperty(globalThis, "document", { configurable: true, value: document });
  t.after(() => {
    stop();
    if (windowDescriptor) Object.defineProperty(globalThis, "window", windowDescriptor); else Reflect.deleteProperty(globalThis, "window");
    if (documentDescriptor) Object.defineProperty(globalThis, "document", documentDescriptor); else Reflect.deleteProperty(globalThis, "document");
    setAccountSession(null, false);
  });
  const fetch = t.mock.method(globalThis, "fetch", async () => Response.json({ error: "session_expired" }, { status: 401 }));
  setAccountSession(account, false);
  const stop = watchAccountSession(() => assert.fail("revocation is an account state, not an offline error"));
  browser.dispatchEvent(Object.assign(new Event("storage"), { key: "keepup.account.changed.v1" }));
  await new Promise((done) => setTimeout(done, 0));
  assert.equal(fetch.mock.callCount(), 1);
  assert.equal(currentSession(), null);
});

test("temporary connection failures retain the known account but never grant fresh authority", async (t) => {
  t.mock.method(globalThis, "fetch", async () => { throw new TypeError("network failure"); });
  setAccountSession(account, false);
  await assert.rejects(refreshAccountSession(), { code: "offline" });
  assert.equal(currentSession()?.user.id, account.user.id);
  setAccountSession(null, false);
});
