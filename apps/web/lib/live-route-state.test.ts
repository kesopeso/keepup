import assert from "node:assert/strict";
import test from "node:test";

import {
  applyMemberLiveEvent,
  parseLiveEvent,
} from "./live-route-state.ts";
import type { MemberSummary, RouteSnapshot } from "./routes-api.ts";

const joinedMember: MemberSummary = {
  id: "member-2",
  routeId: "route-1",
  clientId: "client-2",
  displayName: "Matej",
  transportMode: "train",
  isOwner: false,
  status: "spectating",
  color: "#2563eb",
  joinedAt: "2026-09-18T10:01:00Z",
  leftAt: null,
};

function routeSnapshot(): RouteSnapshot {
  return {
    route: {
      id: "route-1",
      code: "K7P9QD",
      name: "Morning convoy",
      description: "",
      hasPassword: false,
      sharingPolicy: "everyone_can_share",
      status: "active",
      maxTrackingMembers: 10,
      createdAt: "2026-09-18T10:00:00Z",
      closedAt: null,
    },
    members: [
      {
        id: "member-1",
        displayName: "Ana",
        transportMode: "car",
        role: "owner",
        status: "tracking",
        color: "#22c55e",
        joinedAt: "2026-09-18T10:00:00Z",
        leftAt: null,
        paths: [
          {
            id: "segment-1",
            points: [
              {
                latitude: 46.0569,
                longitude: 14.5058,
                recordedAt: "2026-09-18T10:00:30Z",
              },
            ],
          },
        ],
      },
    ],
    viewer: {
      memberId: "member-1",
      role: "owner",
      status: "tracking",
      canStartSharing: false,
      canStopSharing: true,
      canLeaveRoute: false,
      canCloseRoute: true,
      canDeleteRoute: true,
      canEditRoute: true,
    },
  };
}

test("member_joined is parsed and inserted into the snapshot", () => {
  const event = parseLiveEvent(
    JSON.stringify({ type: "member_joined", member: joinedMember }),
  );

  assert.ok(event && event.type === "member_joined");
  const updated = applyMemberLiveEvent(routeSnapshot(), event);

  assert.equal(updated.members.length, 2);
  assert.deepEqual(updated.members[1], {
    id: "member-2",
    displayName: "Matej",
    transportMode: "train",
    role: "member",
    status: "spectating",
    color: "#2563eb",
    joinedAt: "2026-09-18T10:01:00Z",
    leftAt: null,
    paths: [],
  });
});

test("presence events update an existing member without losing path history", () => {
  const snapshot = routeSnapshot();

  for (const statusEvent of [
    ["member_became_stale", "stale"],
    ["member_went_offline", "offline"],
  ] as const) {
    const member = {
      ...joinedMember,
      id: "member-1",
      isOwner: true,
      displayName: "Ana",
      transportMode: "car" as const,
      status: statusEvent[1],
      color: "#22c55e",
      joinedAt: "2026-09-18T10:00:00Z",
    };
    const event = parseLiveEvent(
      JSON.stringify({ type: statusEvent[0], member }),
    );

    assert.ok(event && event.type === statusEvent[0]);
    const updated = applyMemberLiveEvent(snapshot, event);
    assert.equal(updated.members[0].status, statusEvent[1]);
    assert.deepEqual(updated.members[0].paths, snapshot.members[0].paths);
    assert.equal(updated.viewer.status, statusEvent[1]);
  }
});

test("repeated member events upsert rather than duplicate members", () => {
  const first = applyMemberLiveEvent(routeSnapshot(), {
    type: "member_joined",
    member: joinedMember,
  });
  const second = applyMemberLiveEvent(first, {
    type: "member_went_offline",
    member: { ...joinedMember, status: "offline" },
  });

  assert.equal(second.members.length, 2);
  assert.equal(second.members[1].status, "offline");
});

test("malformed member events are rejected", () => {
  assert.equal(
    parseLiveEvent(
      JSON.stringify({
        type: "member_went_offline",
        member: { id: "member-2", status: "offline" },
      }),
    ),
    null,
  );
});
