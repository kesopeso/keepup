import assert from "node:assert/strict";
import test from "node:test";
import { loadGoogleIdentity } from "./google-signin.ts";

test("a failed Google SDK load can be retried without leaving the form stuck", async (t) => {
  const windowDescriptor = Object.getOwnPropertyDescriptor(globalThis, "window");
  const documentDescriptor = Object.getOwnPropertyDescriptor(globalThis, "document");
  const scripts: { onerror?: () => void; onload?: () => void; removed?: boolean; remove: () => void }[] = [];
  const browser = { setTimeout, google: undefined as unknown };
  Object.defineProperty(globalThis, "window", { configurable: true, value: browser });
  Object.defineProperty(globalThis, "document", { configurable: true, value: {
    createElement: () => { const script = { remove() { this.removed = true; }, removed: false }; scripts.push(script); return script; },
    head: { append() {} },
  } });
  t.after(() => {
    if (windowDescriptor) Object.defineProperty(globalThis, "window", windowDescriptor); else Reflect.deleteProperty(globalThis, "window");
    if (documentDescriptor) Object.defineProperty(globalThis, "document", documentDescriptor); else Reflect.deleteProperty(globalThis, "document");
  });
  const failed = loadGoogleIdentity();
  scripts[0].onerror!();
  await assert.rejects(failed, /google_unavailable/);
  assert.equal(scripts[0].removed, true);
  const retried = loadGoogleIdentity();
  assert.equal(scripts.length, 2);
  const identity = {};
  browser.google = { accounts: { id: identity } };
  scripts[1].onload!();
  assert.equal(await retried, identity);
});
