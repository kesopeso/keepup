import type { LocationSample } from './foreground-sharing.ts';

// One owner and serialized native operations prevent a delayed Stop from removing a new session.
export function createLocationTask(options: {
  start: () => Promise<void>;
  stop: () => Promise<void>;
  now?: () => number;
}) {
  type Owner = { sample: (value: LocationSample) => void; error: (message: string) => void };
  let owner: Owner | null = null;
  let operations = Promise.resolve();
  const now = options.now ?? Date.now;
  function enqueue(operation: () => Promise<void>) {
    const result = operations.then(operation);
    operations = result.catch(() => {});
    return result;
  }
  return {
    // Also unregister tasks left by process death. Saved membership never means consent to restart GPS.
    reset() { owner = null; return enqueue(options.stop); },
    async watch(sample: Owner['sample'], error: Owner['error']) {
      const next = { sample, error };
      if (owner) throw new Error('Another location session is already active. Stop sharing first.');
      owner = next;
      try { await enqueue(async () => { await options.stop(); if (owner === next) await options.start(); }); }
      catch (failure) {
        if (owner === next) owner = null;
        await enqueue(options.stop).catch(() => {});
        throw failure;
      }
      return { remove() {
        if (owner !== next) return;
        owner = null;
        void enqueue(options.stop).catch(() => error('Could not stop the location service. Close KeepUp from Android settings.'));
      } };
    },
    deliver(locations: LocationSample[] = [], failure?: string) {
      if (!owner) return enqueue(options.stop);
      if (failure) { owner.error(failure); return Promise.resolve(); }
      // Task delivery can contain a batch after Android scheduling delays. Never replay a backlog.
      const latest = locations.reduce<LocationSample | null>((best, value) =>
        !best || value.timestamp > best.timestamp ? value : best, null);
      if (latest && now() - latest.timestamp <= 10000 && latest.timestamp <= now()) owner.sample(latest);
      return Promise.resolve();
    },
    fail(message: string) { owner?.error(message); },
    isActive() { return owner !== null; },
  };
}
