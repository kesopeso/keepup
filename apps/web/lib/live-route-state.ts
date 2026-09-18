import type {
  MemberSummary,
  PathSegment,
  RoutePoint,
  RouteSnapshot,
  RouteSummary,
  SnapshotMember,
} from "./routes-api";

export type LiveEvent =
  | {
      type: "connection_established";
    }
  | {
      type: "member_joined" | "member_left";
      member: MemberSummary;
    }
  | {
      type: "member_started_sharing";
      member: MemberSummary;
      segment: PathSegment;
    }
  | {
      type: "member_stopped_sharing";
      member: MemberSummary;
    }
  | {
      type: "member_became_stale" | "member_back_online" | "member_went_offline";
      member: MemberSummary;
    }
  | {
      type: "command_ack";
      requestId?: string;
      command?: string;
    }
  | {
      type: "command_rejected";
      requestId?: string;
      command?: string;
      reason?: string;
    }
  | {
      type: "live_connection_rejected";
      reason?: string;
    }
  | {
      type: "position_updated";
      memberId: string;
      segmentId?: string;
      point: RoutePoint;
    }
  | {
      type: "position_rejected";
      error?: string;
    }
  | {
      type: "route_closed";
      route: RouteSummary;
    }
  | {
      type: "message_rejected";
      error?: string;
    };

export type MemberLiveEvent = Extract<LiveEvent, { member: MemberSummary }>;

export function parseLiveEvent(
  payload: string | ArrayBufferLike | Blob,
): LiveEvent | null {
  if (typeof payload !== "string") {
    return null;
  }

  try {
    const event = JSON.parse(payload) as Partial<LiveEvent>;
    if (event.type === "position_updated" && isPositionUpdatedEvent(event)) {
      return event;
    }

    if (isRouteClosedEvent(event)) {
      return event;
    }

    if (
      event.type === "member_started_sharing" &&
      isSharingStartedEvent(event)
    ) {
      return event;
    }

    if (
      (event.type === "member_joined" ||
        event.type === "member_left" ||
        event.type === "member_stopped_sharing" ||
        event.type === "member_became_stale" ||
        event.type === "member_back_online" ||
        event.type === "member_went_offline") &&
      isMemberSummary(event.member)
    ) {
      return event as MemberLiveEvent;
    }

    if (
      event.type === "position_rejected" ||
      event.type === "message_rejected" ||
      event.type === "command_ack" ||
      event.type === "command_rejected" ||
      event.type === "live_connection_rejected" ||
      event.type === "connection_established"
    ) {
      return event as LiveEvent;
    }
  } catch {
    return null;
  }

  return null;
}

export function isMemberLiveEvent(event: LiveEvent): event is MemberLiveEvent {
  return "member" in event;
}

export function applyMemberLiveEvent(
  snapshot: RouteSnapshot,
  event: MemberLiveEvent,
): RouteSnapshot {
  const memberIndex = snapshot.members.findIndex(
    (snapshotMember) => snapshotMember.id === event.member.id,
  );
  const segment =
    event.type === "member_started_sharing" ? event.segment : undefined;
  const members =
    memberIndex >= 0
      ? snapshot.members.map((snapshotMember, index) =>
          index === memberIndex
            ? snapshotMemberFromMemberSummary(
                snapshotMember,
                event.member,
                segment,
              )
            : snapshotMember,
        )
      : [
          ...snapshot.members,
          snapshotMemberFromMemberSummary(
            undefined,
            event.member,
            segment,
          ),
        ];
  const viewer =
    snapshot.viewer.memberId === event.member.id
      ? viewerCapabilitiesForMember(snapshot, event.member)
      : snapshot.viewer;

  return {
    ...snapshot,
    members,
    viewer,
  };
}

function snapshotMemberFromMemberSummary(
  snapshotMember: SnapshotMember | undefined,
  member: MemberSummary,
  segment?: PathSegment,
): SnapshotMember {
  const existingPaths = snapshotMember?.paths ?? [];
  const paths =
    segment && !existingPaths.some((path) => path.id === segment.id)
      ? [...existingPaths, segment]
      : existingPaths;

  return {
    id: member.id,
    displayName: member.displayName,
    transportMode: member.transportMode,
    role: member.isOwner ? "owner" : "member",
    status: member.status,
    color: member.color,
    joinedAt: member.joinedAt,
    leftAt: member.leftAt,
    paths,
  };
}

function viewerCapabilitiesForMember(
  snapshot: RouteSnapshot,
  member: MemberSummary,
) {
  const canUseSharingPolicy =
    snapshot.viewer.role === "owner" ||
    snapshot.route.sharingPolicy === "everyone_can_share";
  const canShare =
    snapshot.route.status === "active" &&
    member.status !== "left" &&
    canUseSharingPolicy;

  return {
    ...snapshot.viewer,
    status: member.status,
    canStartSharing:
      canShare && (member.status === "spectating" || member.status === "stale"),
    canStopSharing:
      canShare && (member.status === "tracking" || member.status === "stale"),
    canLeaveRoute:
      member.status !== "left" &&
      !(snapshot.route.status === "active" && snapshot.viewer.role === "owner"),
  };
}

function isPositionUpdatedEvent(
  event: Partial<LiveEvent>,
): event is Extract<LiveEvent, { type: "position_updated" }> {
  if (
    event.type !== "position_updated" ||
    typeof event.memberId !== "string" ||
    !event.point
  ) {
    return false;
  }

  return (
    typeof event.point.latitude === "number" &&
    typeof event.point.longitude === "number" &&
    typeof event.point.recordedAt === "string"
  );
}

function isSharingStartedEvent(
  event: Partial<LiveEvent>,
): event is Extract<LiveEvent, { type: "member_started_sharing" }> {
  return (
    event.type === "member_started_sharing" &&
    isMemberSummary(event.member) &&
    Boolean(event.segment)
  );
}

function isMemberSummary(member: unknown): member is MemberSummary {
  if (!member || typeof member !== "object") {
    return false;
  }

  const candidate = member as Partial<MemberSummary>;
  return (
    typeof candidate.id === "string" &&
    typeof candidate.displayName === "string" &&
    typeof candidate.transportMode === "string" &&
    typeof candidate.isOwner === "boolean" &&
    typeof candidate.status === "string" &&
    typeof candidate.color === "string" &&
    typeof candidate.joinedAt === "string" &&
    (candidate.leftAt === null || typeof candidate.leftAt === "string")
  );
}

function isRouteSummary(route: unknown): route is RouteSummary {
  if (!route || typeof route !== "object") {
    return false;
  }

  const candidate = route as Partial<RouteSummary>;
  return (
    typeof candidate.id === "string" &&
    typeof candidate.code === "string" &&
    typeof candidate.name === "string" &&
    candidate.status === "closed"
  );
}

function isRouteClosedEvent(
  event: Partial<LiveEvent>,
): event is Extract<LiveEvent, { type: "route_closed" }> {
  return event.type === "route_closed" && isRouteSummary(event.route);
}
