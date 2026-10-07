import type { RouteSnapshot } from '../domain/routes.ts';
import type { ViewingStatus } from '../live/viewing-session.ts';
import { parsePosition } from '../live/viewing-session.ts';

export type LocationSample = { timestamp: number; coords: {
  latitude: number; longitude: number; accuracy: number | null;
  altitude?: number | null; speed?: number | null; heading?: number | null;
} };
export type SharingState = { action: 'start' | 'stop' | null; sharing: boolean; recovery: boolean; error: string | null; settings: boolean };
export class LocationAccessError extends Error {
  settings: boolean;
  constructor(message: string, settings = false) { super(message); this.settings = settings; }
}
const rejectionMessages: Record<string, string> = {
  accuracy_required: 'Your phone has not supplied GPS accuracy. Try outdoors with precise location enabled.',
  accuracy_too_low: 'Location accuracy is too low. Enable precise location and move to an open area.',
  timestamp_required: 'The location timestamp is missing. Stop and restart sharing.',
  timestamp_too_old: 'Your phone supplied an old location. Waiting for a fresh GPS fix.',
  timestamp_in_future: 'Your phone clock is ahead. Enable automatic date and time.',
  duplicate_timestamp: 'This location was already received. Waiting for the next GPS fix.',
  out_of_order_timestamp: 'An older GPS fix arrived late. Waiting for a fresh location.',
  impossible_jump: 'This location jumps too far from the last fix. Waiting for a more reliable location.',
  tracking_limit_reached: 'All tracking slots are occupied. Try again when another member stops sharing.',
  sharing_not_allowed: 'This route only allows its owner to share location.',
  route_closed: 'This route is closed. Location sharing has stopped.',
  invalid_input: 'Your phone supplied an invalid location. Stop and restart sharing.',
};
export function sharingError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return rejectionMessages[message] ?? message;
}
export function positionPayload(sample: LocationSample): Record<string, unknown> | null {
  const c = sample.coords;
  if (!Number.isFinite(c.latitude) || Math.abs(c.latitude) > 90 || !Number.isFinite(c.longitude) || Math.abs(c.longitude) > 180 ||
      c.accuracy === null || !Number.isFinite(c.accuracy) || c.accuracy < 0 || !Number.isFinite(sample.timestamp) ||
      !Number.isFinite(new Date(sample.timestamp).getTime())) return null;
  const payload: Record<string, unknown> = { latitude: c.latitude, longitude: c.longitude, accuracyM: c.accuracy,
    clientRecordedAt: new Date(sample.timestamp).toISOString() };
  if (typeof c.altitude === 'number' && Number.isFinite(c.altitude)) payload.altitudeM = c.altitude;
  if (typeof c.speed === 'number' && Number.isFinite(c.speed) && c.speed >= 0) payload.speedMps = c.speed;
  if (typeof c.heading === 'number' && Number.isFinite(c.heading) && c.heading >= 0 && c.heading <= 360) payload.headingDeg = c.heading;
  return payload;
}

export function createForegroundSharing(options: {
  keepWatchOnDisconnect?: boolean;
  wake?: () => void;
  requestPermission?: () => Promise<void>;
  prepare: (signal: AbortSignal) => Promise<LocationSample>;
  watch: (sample: (value: LocationSample) => void, error: (message: string) => void) => Promise<{ remove: () => void }>;
  command: (type: 'start_sharing' | 'stop_sharing') => Promise<void>;
  send: (payload: Record<string, unknown>) => boolean;
  onState: (state: SharingState) => void;
}) {
  let active = true;
  let version = 0;
  let snapshot: RouteSnapshot | null = null;
  let live: ViewingStatus = 'connecting';
  let intent = false;
  let watch: { remove: () => void } | null = null;
  let watching = false;
  let first: LocationSample | null = null;
  let lastTimestamp = 0;
  let connectedSince = 0;
  let preparing: AbortController | null = null;
  let permissionPending = false;
  let resume: (() => void) | null = null;
  let cancelResume: (() => void) | null = null;
  function waitForLive(): Promise<void> {
    if (live === 'live') return Promise.resolve();
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => { resume = null; cancelResume = null; reject(new Error('Wait for the live connection, then try again.')); }, 10000);
      resume = () => { clearTimeout(timer); resume = null; cancelResume = null; resolve(); };
      cancelResume = () => { clearTimeout(timer); resume = null; cancelResume = null; reject(new Error('Sharing was interrupted. Try again when KeepUp is open.')); };
    });
  }
  let state: SharingState = { action: null, sharing: false, recovery: false, error: null, settings: false };
  function publish(change: Partial<SharingState> = {}) {
    state = { ...state, ...change, sharing: intent, recovery: !intent && snapshot?.route.status === 'active' &&
      ['tracking', 'stale'].includes(snapshot.viewer.status) };
    if (active) options.onState(state);
  }
  function stopWatch() { version++; preparing?.abort(); preparing = null; watch?.remove(); watch = null; watching = false; lastTimestamp = 0; }
  function sample(value: LocationSample) {
    if (active && intent) options.wake?.();
    if (!active || !intent || live !== 'live' || !snapshot || !['tracking', 'stale'].includes(snapshot.viewer.status)) return;
    if (options.keepWatchOnDisconnect && value.timestamp < connectedSince) return;
    const payload = positionPayload(value);
    if (!payload) { publish({ error: 'Your phone supplied an unusable GPS fix. Waiting for a valid location.' }); return; }
    if (value.timestamp <= lastTimestamp) return;
    if (options.send(payload)) lastTimestamp = value.timestamp;
  }
  function reconcile() {
    const canCapture = active && intent && live === 'live' && snapshot?.route.status === 'active' &&
      ['tracking', 'stale'].includes(snapshot.viewer.status);
    if (!canCapture) {
      if (watching && !(options.keepWatchOnDisconnect && active && intent && snapshot?.route.status === 'active')) stopWatch();
      return;
    }
    if (watching) return;
    watching = true;
    const generation = version;
    if (first) { sample(first); first = null; }
    void options.watch((value) => { if (generation === version) sample(value); }, (message) => {
      if (generation !== version || !active) return;
      intent = false; stopWatch(); publish({ error: message });
      // Release the server slot after a native watcher failure when connected.
      void options.command('stop_sharing').catch(() => {});
    }).then((subscription) => {
      if (!active || generation !== version || !intent) subscription.remove();
      else watch = subscription;
    }).catch((error) => {
      if (generation !== version || !active) return;
      intent = false; stopWatch(); publish({ error: sharingError(error), settings: error instanceof LocationAccessError && error.settings });
      void options.command('stop_sharing').catch(() => {});
    });
  }
  async function start() {
    if (!active || state.action || live !== 'live' || (!snapshot?.viewer.canStartSharing && !snapshot?.viewer.canStopSharing)) return;
    const generation = version;
    publish({ action: 'start', error: null, settings: false });
    try {
      permissionPending = !!options.requestPermission;
      try { await options.requestPermission?.(); } finally { permissionPending = false; }
      if (!active || generation !== version) return;
      await waitForLive();
      if (!active || generation !== version) return;
      preparing = new AbortController();
      const location = await options.prepare(preparing.signal);
      preparing = null;
      if (!active || generation !== version || live !== 'live') return;
      if (!positionPayload(location)) throw new Error('Could not obtain a usable GPS fix. Try outdoors with precise location enabled.');
      await options.command('start_sharing');
      if (!active || generation !== version) return;
      intent = true; first = location; publish({ error: null }); reconcile();
    } catch (error) {
      if (active && generation === version) publish({ error: sharingError(error), settings: error instanceof LocationAccessError && error.settings });
    } finally { if (active && generation === version) publish({ action: null }); }
  }
  async function stop() {
    if (!active || state.action) return false;
    intent = false; first = null; stopWatch();
    const generation = version;
    if (live !== 'live') {
      publish({ action: null, error: 'GPS capture stopped. Reconnect to confirm spectator status.' });
      return false;
    }
    publish({ action: 'stop' });
    try {
      await options.command('stop_sharing');
      if (active && generation === version) publish({ error: null, settings: false });
      return true;
    } catch (error) {
      if (active && generation === version) publish({ error: sharingError(error) });
      return false;
    } finally { if (active && generation === version) publish({ action: null }); }
  }
  return {
    start, stop,
    update(result: RouteSnapshot) {
      snapshot = result;
      if (result.route.status !== 'active' || !['tracking', 'stale'].includes(result.viewer.status) && !state.action) {
        const stoppedSharing = intent;
        intent = false; first = null; if (watching) stopWatch();
        publish(stoppedSharing ? { error: null } : {});
      } else publish();
      reconcile();
    },
    connection(status: ViewingStatus) {
      if (status === 'live' && live !== 'live') connectedSince = Date.now();
      live = status;
      if (status === 'paused' && permissionPending) {
        // Android's runtime permission dialog pauses the activity. Wait for foreground before obtaining GPS.
        return;
      }
      if (status === 'live') resume?.();
      if (status === 'paused' || status === 'invalid' || status === 'archive') {
        cancelResume?.();
        intent = false; first = null; stopWatch(); publish({ action: null });
      } else if (status !== 'live' && watching && !options.keepWatchOnDisconnect) stopWatch();
      reconcile();
    },
    event(event: Record<string, unknown>) {
      if (!active || !intent) return;
      if (event.type === 'position_rejected') publish({ error: sharingError(event.error ?? 'Location rejected. Waiting for the next GPS fix.') });
      else if (event.type === 'position_updated' && parsePosition(event)?.memberId === snapshot?.viewer.memberId) publish({ error: null });
    },
    dispose() { active = false; cancelResume?.(); intent = false; first = null; stopWatch(); },
  };
}
