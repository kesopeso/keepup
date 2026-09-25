import assert from "node:assert/strict";
import test from "node:test";

import {
  connectRouteLiveSocket,
  type LiveSocket,
  type LiveSocketEventMap,
} from "./live-connection.ts";

class FakeSocket implements LiveSocket {
  readonly sent: string[] = [];
  readyState: number = WebSocket.OPEN;
  private readonly listeners = new Map<
    keyof LiveSocketEventMap,
    Array<(event: Event) => void>
  >();

  addEventListener<K extends keyof LiveSocketEventMap>(
    type: K,
    listener: (event: LiveSocketEventMap[K]) => void,
  ) {
    const listeners = this.listeners.get(type) ?? [];
    listeners.push(listener as (event: Event) => void);
    this.listeners.set(type, listeners);
  }

  send(payload: string) {
    this.sent.push(payload);
  }

  close() {
    this.readyState = WebSocket.CLOSED;
  }

  emit(type: keyof LiveSocketEventMap, data = "") {
    const event =
      type === "message"
        ? ({ data } as MessageEvent<string>)
        : new Event(type);
    for (const listener of this.listeners.get(type) ?? []) {
      listener(event);
    }
  }
}

test("unexpected close reconnects and authenticates the replacement socket", () => {
  const sockets: FakeSocket[] = [];
  const scheduled: Array<() => void> = [];
  const connection = connectRouteLiveSocket({
    memberToken: "member-token",
    url: "wss://example.test/ws",
    createSocket: () => {
      const socket = new FakeSocket();
      sockets.push(socket);
      return socket;
    },
    onMessage: () => {},
    onSocketChange: () => {},
    scheduleReconnect: (callback) => {
      scheduled.push(callback);
      return callback;
    },
    cancelReconnect: () => {},
  });

  assert.equal(connection.isRecovering(), false);
  sockets[0].emit("open");
  assert.deepEqual(JSON.parse(sockets[0].sent[0]), {
    type: "authenticate",
    memberToken: "member-token",
  });

  sockets[0].emit("close");
  assert.equal(connection.isRecovering(), true);
  assert.equal(scheduled.length, 1);
  scheduled[0]();
  assert.equal(sockets.length, 2);

  sockets[1].emit("open");
  connection.markEstablished();
  assert.equal(connection.isRecovering(), false);
  assert.deepEqual(JSON.parse(sockets[1].sent[0]), {
    type: "authenticate",
    memberToken: "member-token",
  });

  connection.stop();
});

test("intentional stop cancels reconnect and leaves no active socket", () => {
  const socket = new FakeSocket();
  const scheduled: Array<() => void> = [];
  let cancelled = false;
  const connection = connectRouteLiveSocket({
    memberToken: "member-token",
    url: "wss://example.test/ws",
    createSocket: () => socket,
    onMessage: () => {},
    onSocketChange: () => {},
    scheduleReconnect: (callback) => {
      scheduled.push(callback);
      return callback;
    },
    cancelReconnect: () => {
      cancelled = true;
    },
  });

  socket.emit("close");
  connection.stop();

  assert.equal(cancelled, true);
  scheduled[0]();
  assert.equal(connection.current(), null);
});
