import assert from "node:assert/strict";
import test from "node:test";
import { shareRouteLink } from "./route-share.ts";

test("route and dashboard sharing use native share, silent cancellation, clipboard, and manual fallback", async () => {
  Object.defineProperty(globalThis, "window", { configurable: true, value: { location: { origin: "https://keepup.test" } } });
  let shared: object | undefined;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: { share: async (value: object) => { shared = value; } } });
  assert.equal(await shareRouteLink("ABC123", "Trip"), null);
  assert.deepEqual(shared, { title: "Trip", url: "https://keepup.test/routes/ABC123" });
  Object.defineProperty(navigator, "share", { configurable: true, value: async () => { throw new DOMException("Cancelled", "AbortError"); } });
  assert.equal(await shareRouteLink("ABC123", "Trip"), null);
  let copied = "";
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: { clipboard: { writeText: async (value: string) => { copied = value; } } } });
  assert.equal((await shareRouteLink("ABC123", "Trip"))?.tone, "success");
  assert.equal(copied, "https://keepup.test/routes/ABC123");
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: {} });
  assert.equal((await shareRouteLink("ABC123", "Trip"))?.manualUrl, "https://keepup.test/routes/ABC123");
});
