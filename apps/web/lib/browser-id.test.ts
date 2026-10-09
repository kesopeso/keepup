import assert from "node:assert/strict";
import test from "node:test";

import { createBrowserId } from "./browser-id.ts";
import { getOrCreateClientId } from "./identity-storage.ts";

const uuidV4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

test("uses native UUID generation when available", (t) => {
  const crypto = globalThis.crypto;
  const expected = "d9a4b034-d695-407c-bf4d-af07e2d04b50";
  const generate = t.mock.method(crypto, "randomUUID", () => expected);
  assert.equal(createBrowserId(), expected);
  assert.equal(generate.mock.callCount(), 1);
});

test("HTTP sharing IDs work without randomUUID and retain browser identity", (t) => {
  const crypto = globalThis.crypto;
  const cryptoDescriptor = Object.getOwnPropertyDescriptor(globalThis, "crypto")!;
  const windowDescriptor = Object.getOwnPropertyDescriptor(globalThis, "window");
  t.after(() => {
    Object.defineProperty(globalThis, "crypto", cryptoDescriptor);
    if (windowDescriptor) Object.defineProperty(globalThis, "window", windowDescriptor);
    else Reflect.deleteProperty(globalThis, "window");
  });
  // Match HTTP IP access: randomUUID is absent, getRandomValues still works.
  Object.defineProperty(globalThis, "crypto", {
    configurable: true,
    value: { getRandomValues: crypto.getRandomValues.bind(crypto) },
  });
  const values = new Map<string, string>();
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
      localStorage: {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, value),
      },
    },
  });

  const clientId = getOrCreateClientId();
  assert.match(clientId, uuidV4);
  assert.equal(getOrCreateClientId(), clientId);
  const requestIds = new Set(Array.from({ length: 100 }, () => createBrowserId()));
  assert.equal(requestIds.size, 100);
  for (const id of requestIds) assert.match(id, uuidV4);
  assert.equal(requestIds.has(clientId), false);

  values.set("keepup.clientId", "client_existing-device");
  assert.equal(getOrCreateClientId(), "client_existing-device");
});

test("retains the existing fallback when Web Crypto is unavailable", (t) => {
  const descriptor = Object.getOwnPropertyDescriptor(globalThis, "crypto")!;
  t.after(() => Object.defineProperty(globalThis, "crypto", descriptor));
  Object.defineProperty(globalThis, "crypto", { configurable: true, value: undefined });
  assert.match(createBrowserId(), /^client_[a-z0-9]+_[a-z0-9]+$/);
});
