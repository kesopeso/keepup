import assert from "node:assert/strict";
import test from "node:test";

import {
  createDevelopmentNavigationService,
  type NavigationPosition,
} from "./navigation-service.ts";

test("development sharing starts and restarts without browser GPS", async (t) => {
  t.mock.timers.enable({ apis: ["setInterval", "Date"], now: 1_800_000_000_000 });
  const service = createDevelopmentNavigationService();
  // Node has no browser geolocation. A permission request must still succeed.
  await service.requestPermission();
  const samples: NavigationPosition[] = [];
  const errors: Error[] = [];
  const stop = service.watchPosition(
    (sample) => samples.push(sample),
    (error) => errors.push(error),
  );
  assert.equal(samples.length, 1, "emit the first sample immediately");
  t.mock.timers.tick(2_000);
  assert.equal(samples.length, 2);
  assert.notDeepEqual(
    [samples[0].latitude, samples[0].longitude],
    [samples[1].latitude, samples[1].longitude],
  );
  stop();
  t.mock.timers.tick(20_000);
  assert.equal(samples.length, 2, "stopping cancels simulated movement");

  await service.requestPermission();
  const stopAgain = service.watchPosition(
    (sample) => samples.push(sample),
    (error) => errors.push(error),
  );
  t.after(stopAgain);
  assert.equal(samples.length, 3, "restart emits immediately");
  assert.equal(samples[2].latitude, samples[1].latitude);
  assert.equal(samples[2].longitude, samples[1].longitude);
  assert.ok(
    Date.parse(samples[2].clientRecordedAt) >
      Date.parse(samples[1].clientRecordedAt),
  );
  t.mock.timers.tick(2_000);
  assert.equal(samples.length, 4, "restart resumes simulated movement");
  for (const sample of samples) {
    assert.equal(sample.accuracyM, 5);
    assert.ok(Number.isFinite(sample.latitude));
    assert.ok(Number.isFinite(sample.longitude));
  }
  assert.deepEqual(errors, []);
});
