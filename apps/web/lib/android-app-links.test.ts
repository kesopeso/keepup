import assert from "node:assert/strict";
import test from "node:test";
import { GET } from "../app/.well-known/assetlinks.json/route.ts";

test("Android association grants only explicitly configured signing certificates", async () => {
  const previous = process.env.ANDROID_APP_LINK_SHA256_FINGERPRINTS;
  const fingerprint = Array(32).fill("ab").join(":");
  try {
    delete process.env.ANDROID_APP_LINK_SHA256_FINGERPRINTS;
    assert.deepEqual(await GET().json(), []);

    process.env.ANDROID_APP_LINK_SHA256_FINGERPRINTS = ` ${fingerprint},${fingerprint} `;
    const response = GET();
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("cache-control"), "no-store");
    assert.deepEqual(await response.json(), [{
      relation: ["delegate_permission/common.handle_all_urls"],
      target: { namespace: "android_app", package_name: "eu.kesopeso.keepup", sha256_cert_fingerprints: [fingerprint.toUpperCase()] },
    }]);

    process.env.ANDROID_APP_LINK_SHA256_FINGERPRINTS = `${fingerprint},not-a-certificate`;
    const invalid = GET();
    assert.equal(invalid.status, 503);
    assert.deepEqual(await invalid.json(), { error: "invalid_app_link_configuration" });
  } finally {
    if (previous === undefined) delete process.env.ANDROID_APP_LINK_SHA256_FINGERPRINTS;
    else process.env.ANDROID_APP_LINK_SHA256_FINGERPRINTS = previous;
  }
});
