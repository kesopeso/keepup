import type { Membership, RoutePoint, RouteSnapshot } from '../domain/routes.ts';

export type ViewingStatus = 'connecting' | 'live' | 'reconnecting' | 'paused' | 'archive' | 'unavailable' | 'invalid';
type SocketEvents = { open: Event; message: MessageEvent; close: CloseEvent; error: Event };
export interface ViewingSocket {
  addEventListener<K extends keyof SocketEvents>(type: K, listener: (event: SocketEvents[K]) => void): void;
  send(payload: string): void;
  close(): void;
}

type PositionEvent = { type: 'position_updated'; memberId: string; segmentId: string; point: RoutePoint };
function record(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object';
}
export function parsePosition(event: unknown): PositionEvent | null {
  if (!record(event) || event.type !== 'position_updated' || typeof event.memberId !== 'string' ||
      typeof event.segmentId !== 'string' || !record(event.point)) return null;
  const point = event.point;
  if (typeof point.latitude !== 'number' || !Number.isFinite(point.latitude) || Math.abs(point.latitude) > 90 ||
      typeof point.longitude !== 'number' || !Number.isFinite(point.longitude) || Math.abs(point.longitude) > 180 ||
      typeof point.recordedAt !== 'string' || !Number.isFinite(Date.parse(point.recordedAt)) ||
      typeof point.seq !== 'number' || !Number.isSafeInteger(point.seq) || point.seq < 1) return null;
  return event as PositionEvent;
}

export function applyPosition(snapshot: RouteSnapshot, event: PositionEvent): RouteSnapshot {
  return { ...snapshot, members: snapshot.members.map((member) => {
    if (member.id !== event.memberId) return member;
    const path = member.paths.find((path) => path.id === event.segmentId);
    if (!path) return member;
    if (path.points.some((point) => point.seq === event.point.seq)) return member;
    return { ...member, paths: member.paths.map((segment) => segment !== path ? segment : {
      ...segment, points: [...segment.points, event.point].sort((a, b) => (a.seq ?? 0) - (b.seq ?? 0)),
    }) };
  }) };
}

export function viewingSocketUrl(baseUrl: string): string {
  const url = new URL(baseUrl);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  url.pathname = url.pathname.replace(/\/+$/, '').replace(/\/api$/, '') + '/ws';
  url.search = ''; url.hash = '';
  return url.toString();
}

const snapshotEvents = new Set(['member_joined', 'member_left', 'member_started_sharing',
  'member_stopped_sharing', 'member_became_stale', 'member_back_online', 'member_went_offline',
  'route_updated', 'route_closed']);

export function createViewingSession(options: {
  url: string; member: Membership;
  load: (signal: AbortSignal) => Promise<RouteSnapshot>;
  onSnapshot: (snapshot: RouteSnapshot) => void;
  onStatus: (status: ViewingStatus) => void;
  onError: (message: string | null, invalid: boolean) => void;
  isInvalid: (error: unknown) => boolean;
  createSocket?: (url: string) => ViewingSocket;
  schedule?: (callback: () => void, delay: number) => ReturnType<typeof setTimeout>;
  cancel?: (timer: ReturnType<typeof setTimeout>) => void;
}) {
  const schedule = options.schedule ?? setTimeout;
  const cancel = options.cancel ?? clearTimeout;
  let active = true;
  let foreground = true;
  let snapshot: RouteSnapshot | null = null;
  let socket: ViewingSocket | null = null;
  let established = false;
  let retry = 0;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let request: AbortController | null = null;
  let dirty = false;
  let buffered: PositionEvent[] = [];
  let duplicate = false;
  let watchdog: ReturnType<typeof setTimeout> | null = null;

  function closeSocket() {
    established = false;
    if (watchdog) { cancel(watchdog); watchdog = null; }
    const previous = socket; socket = null; previous?.close();
  }
  function retryLater() {
    if (!active || !foreground || timer || snapshot?.route.status === 'closed') return;
    options.onStatus('reconnecting');
    timer = schedule(() => { timer = null; void refresh(); }, Math.min(1000 * 2 ** retry++, 30000));
  }
  function connect() {
    if (!active || !foreground || socket || snapshot?.route.status !== 'active') return;
    options.onStatus('connecting');
    duplicate = false;
    let next: ViewingSocket;
    try { next = (options.createSocket ?? ((url) => new WebSocket(url)))(options.url); }
    catch { retryLater(); return; }
    socket = next;
    const current = () => active && foreground && socket === next;
    watchdog = schedule(() => { if (current() && !established) { closeSocket(); retryLater(); } }, 10000);
    next.addEventListener('open', () => {
      if (!current()) return;
      try { next.send(JSON.stringify({ type: 'authenticate', memberToken: options.member.memberToken })); }
      catch { closeSocket(); retryLater(); }
    });
    next.addEventListener('message', (message) => {
      if (!current() || typeof message.data !== 'string') return;
      let event: unknown;
      try { event = JSON.parse(message.data); } catch { return; }
      if (!record(event)) return;
      if (event.type === 'connection_established') {
        if (watchdog) cancel(watchdog); established = true; retry = 0;
        // Subscribe before fetching, so no updates can be lost between snapshot and socket.
        void refresh();
      } else if (event.type === 'live_connection_rejected') {
        duplicate = event.reason === 'already_active_connection';
        options.onError(duplicate ? 'This membership is connected elsewhere. Waiting to reconnect.' : null, false);
        closeSocket(); retryLater();
      } else if (event.type === 'position_updated') {
        const position = parsePosition(event);
        if (!position || !snapshot) return;
        if (request) {
          buffered.push(position);
          if (buffered.length > 1000) { closeSocket(); request.abort(); request = null; buffered = []; retryLater(); }
        } else if (!snapshot.members.some((member) => member.id === position.memberId && member.paths.some((path) => path.id === position.segmentId))) {
          void refresh();
        } else {
          snapshot = applyPosition(snapshot, position); options.onSnapshot(snapshot);
        }
      } else if (typeof event.type === 'string' && snapshotEvents.has(event.type)) {
        void refresh();
      }
    });
    next.addEventListener('close', (event) => {
      if (!current()) return;
      if (watchdog) { cancel(watchdog); watchdog = null; }
      socket = null; established = false;
      if (event.code === 1008 && !duplicate) void refresh();
      else retryLater();
    });
    next.addEventListener('error', () => { if (current()) { if (watchdog) cancel(watchdog); closeSocket(); retryLater(); } });
  }

  async function refresh() {
    if (!active || !foreground) return;
    if (request) { dirty = true; return; }
    if (timer) { cancel(timer); timer = null; }
    const controller = new AbortController(); request = controller; buffered = []; dirty = false;
    try {
      let result = await options.load(controller.signal);
      if (!active || controller.signal.aborted || request !== controller) return;
      for (const event of buffered) result = applyPosition(result, event);
      snapshot = result;
      options.onSnapshot(result); options.onError(null, false);
      if (result.route.status === 'closed') { closeSocket(); options.onStatus('archive'); }
      else if (established) options.onStatus('live');
      else connect();
    } catch (error) {
      if (!active || controller.signal.aborted || request !== controller) return;
      const invalid = options.isInvalid(error);
      options.onError(error instanceof Error ? error.message : 'Could not update the route.', invalid);
      options.onStatus(invalid ? 'invalid' : 'unavailable');
      closeSocket();
      if (invalid) { active = false; }
      else retryLater();
    } finally {
      if (request === controller) {
        request = null; buffered = [];
        if (dirty && active && foreground) void refresh();
      }
    }
  }

  void refresh();
  return {
    refresh,
    setForeground(value: boolean) {
      if (!active || value === foreground) return;
      foreground = value;
      if (!value) {
        closeSocket(); request?.abort(); request = null; buffered = [];
        if (timer) { cancel(timer); timer = null; }
        options.onStatus('paused');
      } else void refresh();
    },
    stop() {
      active = false; closeSocket(); request?.abort(); request = null;
      if (timer) cancel(timer);
    },
  };
}
