export type LiveSocketEventMap = {
  open: Event;
  message: MessageEvent<string>;
  close: CloseEvent;
  error: Event;
};

export interface LiveSocket {
  readonly readyState: number;
  addEventListener<K extends keyof LiveSocketEventMap>(
    type: K,
    listener: (event: LiveSocketEventMap[K]) => void,
  ): void;
  send(payload: string): void;
  close(): void;
}

type ReconnectHandle = unknown;

type RouteLiveSocketOptions = {
  url: string;
  memberToken: string;
  createSocket?: (url: string) => LiveSocket;
  onMessage: (event: MessageEvent<string>) => void;
  onSocketChange: (socket: LiveSocket | null) => void;
  onDisconnected?: () => void;
  onError?: () => void;
  scheduleReconnect?: (
    callback: () => void,
    delayMs: number,
  ) => ReconnectHandle;
  cancelReconnect?: (handle: ReconnectHandle) => void;
};

export type RouteLiveSocketConnection = {
  current: () => LiveSocket | null;
  isRecovering: () => boolean;
  markEstablished: () => void;
  stop: () => void;
};

const reconnectBaseDelayMs = 1_000;
const reconnectMaxDelayMs = 30_000;

export function connectRouteLiveSocket(
  options: RouteLiveSocketOptions,
): RouteLiveSocketConnection {
  const createSocket =
    options.createSocket ?? ((url: string) => new WebSocket(url));
  const scheduleReconnect =
    options.scheduleReconnect ??
    ((callback: () => void, delayMs: number) =>
      setTimeout(callback, delayMs));
  const cancelReconnect =
    options.cancelReconnect ??
    ((handle: ReconnectHandle) =>
      clearTimeout(handle as ReturnType<typeof setTimeout>));

  let active = true;
  let recovering = false;
  let reconnectAttempt = 0;
  let reconnectHandle: ReconnectHandle | null = null;
  let socket: LiveSocket | null = null;

  function connect() {
    if (!active) {
      return;
    }

    reconnectHandle = null;
    const nextSocket = createSocket(options.url);
    socket = nextSocket;
    options.onSocketChange(nextSocket);

    nextSocket.addEventListener("open", () => {
      if (!active || socket !== nextSocket) {
        return;
      }

      nextSocket.send(
        JSON.stringify({
          type: "authenticate",
          memberToken: options.memberToken,
        }),
      );
    });

    nextSocket.addEventListener("message", (event) => {
      if (active && socket === nextSocket) {
        options.onMessage(event);
      }
    });

    nextSocket.addEventListener("error", () => {
      if (active && socket === nextSocket) {
        options.onError?.();
      }
    });

    nextSocket.addEventListener("close", () => {
      if (!active || socket !== nextSocket) {
        return;
      }

      socket = null;
      recovering = true;
      options.onSocketChange(null);
      options.onDisconnected?.();

      const delayMs = reconnectDelayMs(reconnectAttempt);
      reconnectAttempt += 1;
      reconnectHandle = scheduleReconnect(connect, delayMs);
    });
  }

  connect();

  return {
    current: () => socket,
    isRecovering: () => recovering,
    markEstablished() {
      recovering = false;
      reconnectAttempt = 0;
    },
    stop() {
      if (!active) {
        return;
      }

      active = false;
      if (reconnectHandle !== null) {
        cancelReconnect(reconnectHandle);
        reconnectHandle = null;
      }

      const activeSocket = socket;
      socket = null;
      options.onSocketChange(null);
      activeSocket?.close();
    },
  };
}

export function reconnectDelayMs(attempt: number) {
  return Math.min(
    reconnectBaseDelayMs * 2 ** Math.max(0, attempt),
    reconnectMaxDelayMs,
  );
}
