"use client";

import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { RouteMap } from "../../components/route-map";
import {
  clearRouteAuth,
  getProfile,
  getRouteAuth,
  saveProfile,
  saveRouteAuth,
  transportModes,
  type TransportMode,
} from "../../../lib/identity-storage";
import {
  ApiError,
  getRouteAccess,
  getRouteSnapshot,
  joinRoute,
  routeWebSocketUrl,
  startSharing,
  stopSharing,
  type RouteAccess,
  type RoutePoint,
  type RouteSnapshot,
  type SnapshotMember,
} from "../../../lib/routes-api";
import {
  appendLiveRoutePoint,
  routeSnapshotToMapState,
} from "../../../lib/map/snapshot-map-state";

const transportLabels: Record<TransportMode, string> = {
  walking: "Walking",
  bicycle: "Bicycle",
  car: "Car",
  bus: "Bus",
  train: "Train",
  boat: "Boat",
  airplane: "Airplane",
};

export function JoinRouteScreen({ code }: { code: string }) {
  const [access, setAccess] = useState<RouteAccess | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [transportMode, setTransportMode] = useState<TransportMode>("car");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isJoining, setIsJoining] = useState(false);
  const [memberToken, setMemberToken] = useState("");
  const [snapshot, setSnapshot] = useState<RouteSnapshot | null>(null);

  useEffect(() => {
    let isMounted = true;

    const profile = getProfile();
    setDisplayName(profile.displayName);
    setTransportMode(profile.transportMode);

    const routeAuth = getRouteAuth(code);
    if (routeAuth?.memberToken) {
      setMemberToken(routeAuth.memberToken);
      getRouteSnapshot(code, routeAuth.memberToken)
        .then((routeSnapshot) => {
          if (!isMounted) {
            return;
          }

          setSnapshot(routeSnapshot);
          setIsLoading(false);
        })
        .catch((caughtError) => {
          if (!isMounted) {
            return;
          }

          if (
            caughtError instanceof ApiError &&
            caughtError.code === "unauthorized"
          ) {
            clearRouteAuth(code);
            setMemberToken("");
            loadRouteAccess(
              code,
              () => isMounted,
              setAccess,
              setError,
              setIsLoading,
            );
            return;
          }

          setError(
            caughtError instanceof Error
              ? caughtError.message
              : "Could not load this route.",
          );
          setIsLoading(false);
        });
      return;
    }

    loadRouteAccess(
      code,
      () => isMounted,
      setAccess,
      setError,
      setIsLoading,
    );

    return () => {
      isMounted = false;
    };
  }, [code]);

  const canJoin = useMemo(
    () =>
      displayName.trim() !== "" &&
      (!access?.requiresPassword || password !== "") &&
      !isJoining,
    [access?.requiresPassword, displayName, isJoining, password],
  );

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");

    if (!canJoin) {
      setError("Fill in the required details to join.");
      return;
    }

    setIsJoining(true);

    try {
      const profile = saveProfile({
        displayName,
        transportMode,
      });

      const result = await joinRoute(code, {
        clientId: profile.clientId,
        displayName: profile.displayName,
        transportMode: profile.transportMode,
        password,
      });

      saveRouteAuth(result.route.code, {
        memberToken: result.memberToken,
      });
      setMemberToken(result.memberToken);
      const routeSnapshot = await getRouteSnapshot(
        result.route.code,
        result.memberToken,
      );
      setSnapshot(routeSnapshot);
    } catch (caughtError) {
      setError(
        caughtError instanceof Error
          ? caughtError.message
          : "Could not join this route.",
      );
    } finally {
      setIsJoining(false);
    }
  }

  if (isLoading) {
    return (
      <section className="route-shell">
        <RouteHeader code={code} label="Route" />
        <p className="route-status">Loading route...</p>
      </section>
    );
  }

  if (snapshot) {
    return (
      <RouteSnapshotShell
        code={code}
        memberToken={memberToken}
        onSnapshotChange={setSnapshot}
        snapshot={snapshot}
      />
    );
  }

  if (!access) {
    return (
      <section className="route-shell">
        <RouteHeader code={code} label="Route" />
        <div className="route-panel">
          <p className="form-error" role="alert">
            {error || "Could not load this route."}
          </p>
        </div>
      </section>
    );
  }

  return (
    <form className="route-form" onSubmit={handleSubmit}>
      <RouteHeader code={access.code} label="Join route" title={access.name} />

      {access.description ? (
        <p className="route-description">{access.description}</p>
      ) : null}

      <div className="route-meta">
        <span>{access.status === "closed" ? "Closed archive" : "Active"}</span>
        <span>
          {access.sharingPolicy === "everyone_can_share"
            ? "Everyone can share"
            : "Joiners view only"}
        </span>
        {access.requiresPassword ? <span>Password required</span> : null}
      </div>

      <div className="field-grid">
        <label className="field">
          <span>Your name</span>
          <input
            autoComplete="name"
            name="displayName"
            onChange={(event) => setDisplayName(event.target.value)}
            placeholder="Ana"
            required
            value={displayName}
          />
        </label>

        <label className="field">
          <span>Transport</span>
          <select
            name="transportMode"
            onChange={(event) =>
              setTransportMode(event.target.value as TransportMode)
            }
            value={transportMode}
          >
            {transportModes.map((mode) => (
              <option key={mode} value={mode}>
                {transportLabels[mode]}
              </option>
            ))}
          </select>
        </label>
      </div>

      {access.requiresPassword ? (
        <label className="field">
          <span>Password</span>
          <input
            autoComplete="current-password"
            name="password"
            onChange={(event) => setPassword(event.target.value)}
            required
            type="password"
            value={password}
          />
        </label>
      ) : null}

      {error ? (
        <p className="form-error" role="alert">
          {error}
        </p>
      ) : null}

      <button className="primary-action" disabled={!canJoin} type="submit">
        {isJoining ? "Joining..." : "Join route"}
      </button>
    </form>
  );
}

function loadRouteAccess(
  code: string,
  shouldUpdate: () => boolean,
  setAccess: (access: RouteAccess) => void,
  setError: (error: string) => void,
  setIsLoading: (isLoading: boolean) => void,
) {
  getRouteAccess(code)
    .then((routeAccess) => {
      if (!shouldUpdate()) {
        return;
      }

      setAccess(routeAccess);
      setIsLoading(false);
    })
    .catch((caughtError) => {
      if (!shouldUpdate()) {
        return;
      }

      setError(
        caughtError instanceof Error
          ? caughtError.message
          : "Could not load this route.",
      );
      setIsLoading(false);
    });
}

function RouteSnapshotShell({
  code,
  memberToken,
  onSnapshotChange,
  snapshot,
}: {
  code: string;
  memberToken: string;
  onSnapshotChange: (snapshot: RouteSnapshot) => void;
  snapshot: RouteSnapshot;
}) {
  const [sharingAction, setSharingAction] = useState<"start" | "stop" | null>(
    null,
  );
  const [sharingError, setSharingError] = useState("");
  const [liveTrackingError, setLiveTrackingError] = useState("");
  const websocketRef = useRef<WebSocket | null>(null);
  const [mapState, setMapState] = useState(() => routeSnapshotToMapState(snapshot));
  const sortedMembers = [...snapshot.members].sort(compareMembers);
  const canUseSharingControl =
    memberToken !== "" &&
    snapshot.route.status === "active" &&
    !sharingAction &&
    (snapshot.viewer.canStartSharing || snapshot.viewer.canStopSharing);
  const sharingControlLabel = snapshot.viewer.canStopSharing
    ? "Stop sharing"
    : "Start sharing";
  const sharingControlBusyLabel =
    sharingAction === "stop" ? "Stopping..." : "Starting...";
  const isViewerTracking =
    memberToken !== "" &&
    snapshot.route.status === "active" &&
    snapshot.viewer.status === "tracking";

  useEffect(() => {
    setMapState(routeSnapshotToMapState(snapshot));
  }, [snapshot]);

  useEffect(() => {
    if (memberToken === "" || snapshot.route.status !== "active") {
      return;
    }

    let isCurrent = true;
    const socket = new WebSocket(routeWebSocketUrl);
    websocketRef.current = socket;

    socket.addEventListener("open", () => {
      socket.send(
        JSON.stringify({
          type: "authenticate",
          memberToken,
        }),
      );
    });

    socket.addEventListener("message", (event) => {
      const liveEvent = parseLiveEvent(event.data);
      if (!liveEvent || !isCurrent) {
        return;
      }

      if (liveEvent.type === "position_updated") {
        setMapState((current) =>
          appendLiveRoutePoint(current, {
            memberId: liveEvent.memberId,
            segmentId: liveEvent.segmentId,
            point: liveEvent.point,
          }),
        );
        return;
      }

      if (liveEvent.type === "position_rejected") {
        setLiveTrackingError(positionRejectedMessage(liveEvent.error));
      }
    });

    socket.addEventListener("close", () => {
      if (websocketRef.current === socket) {
        websocketRef.current = null;
      }
    });

    socket.addEventListener("error", () => {
      if (isCurrent) {
        setLiveTrackingError("Live route connection is unavailable.");
      }
    });

    return () => {
      isCurrent = false;
      if (websocketRef.current === socket) {
        websocketRef.current = null;
      }
      socket.close();
    };
  }, [memberToken, snapshot.route.status]);

  useEffect(() => {
    if (!isViewerTracking) {
      return;
    }

    if (!("geolocation" in navigator)) {
      setLiveTrackingError("Location sharing is not available in this browser.");
      return;
    }

    setLiveTrackingError("");
    const watchId = navigator.geolocation.watchPosition(
      (position) => {
        const socket = websocketRef.current;
        if (!socket || socket.readyState !== WebSocket.OPEN) {
          return;
        }

        socket.send(JSON.stringify(positionUpdatePayload(position)));
      },
      () => {
        setLiveTrackingError("Location sharing needs browser location access.");
      },
      {
        enableHighAccuracy: true,
        maximumAge: 5_000,
        timeout: 20_000,
      },
    );

    return () => {
      navigator.geolocation.clearWatch(watchId);
    };
  }, [isViewerTracking]);

  async function handleSharingControl() {
    if (!canUseSharingControl) {
      return;
    }

    setSharingError("");
    const shouldStartSharing = snapshot.viewer.canStartSharing;
    setSharingAction(shouldStartSharing ? "start" : "stop");

    try {
      if (shouldStartSharing) {
        await startSharing(code, memberToken);
      } else {
        await stopSharing(code, memberToken);
      }

      const nextSnapshot = await getRouteSnapshot(code, memberToken);
      onSnapshotChange(nextSnapshot);
    } catch (caughtError) {
      if (
        caughtError instanceof ApiError &&
        caughtError.code === "unauthorized"
      ) {
        clearRouteAuth(code);
      }

      setSharingError(
        caughtError instanceof Error
          ? caughtError.message
          : "Could not update sharing.",
      );
    } finally {
      setSharingAction(null);
    }
  }

  return (
    <section className="route-screen">
      <header className="route-topbar">
        <div className="route-title-block">
          <p className="eyebrow">
            {snapshot.route.status === "closed" ? "Archive" : "Route"}
          </p>
          <h1>{snapshot.route.name}</h1>
          <p className="route-code">{snapshot.route.code}</p>
        </div>

        <div className="route-meta route-topbar-meta">
          <span>{snapshot.route.status === "closed" ? "Closed" : "Active"}</span>
          <span>{snapshot.members.length} members</span>
        </div>
      </header>

      <RouteMap state={mapState} />

      <aside className="member-sheet" aria-label="Route members">
        <div className="sheet-handle" aria-hidden="true" />

        <div className="sheet-section">
          <div className="sheet-heading">
            <h2>Route</h2>
            <span>
              {snapshot.route.sharingPolicy === "everyone_can_share"
                ? "Everyone can share"
                : "Joiners view only"}
            </span>
          </div>

          {snapshot.route.description ? (
            <p className="route-description">{snapshot.route.description}</p>
          ) : null}
        </div>

        <div className="sheet-section">
          <div className="sheet-heading">
            <h2>Your access</h2>
            <span>{formatStatus(snapshot.viewer.status)}</span>
          </div>
          <div className="capability-grid">
            <CapabilityLabel
              label="Role"
              value={formatRole(snapshot.viewer.role)}
            />
            <CapabilityLabel
              label="Can share"
              value={snapshot.viewer.canStartSharing ? "Yes" : "No"}
            />
            <CapabilityLabel
              label="Can leave"
              value={snapshot.viewer.canLeaveRoute ? "Yes" : "No"}
            />
            <CapabilityLabel
              label="Can manage"
              value={snapshot.viewer.canEditRoute ? "Yes" : "No"}
            />
          </div>

          <button
            className="sharing-action"
            disabled={!canUseSharingControl}
            onClick={handleSharingControl}
            type="button"
          >
            {sharingAction ? sharingControlBusyLabel : sharingControlLabel}
          </button>

          {sharingError ? (
            <p className="form-error" role="alert">
              {sharingError}
            </p>
          ) : null}

          {liveTrackingError ? (
            <p className="form-error" role="alert">
              {liveTrackingError}
            </p>
          ) : null}
        </div>

        <div className="sheet-section">
          <div className="sheet-heading">
            <h2>Members</h2>
            <span>{sortedMembers.length}</span>
          </div>
          <div className="member-list">
            {sortedMembers.map((member) => (
              <MemberRow key={member.id} member={member} />
            ))}
          </div>
        </div>
      </aside>
    </section>
  );
}

function CapabilityLabel({ label, value }: { label: string; value: string }) {
  return (
    <div className="capability-item">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function MemberRow({ member }: { member: SnapshotMember }) {
  return (
    <article className="member-row">
      <span
        aria-hidden="true"
        className="member-color"
        style={{ backgroundColor: member.color }}
      />
      <div>
        <h3>{member.displayName}</h3>
        <p>
          {formatRole(member.role)} · {formatStatus(member.status)} ·{" "}
          {formatTransportMode(member.transportMode)}
        </p>
      </div>
    </article>
  );
}

type LiveEvent =
  | {
      type: "connection_established";
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
      type: "message_rejected";
      error?: string;
    };

function parseLiveEvent(payload: string | ArrayBufferLike | Blob): LiveEvent | null {
  if (typeof payload !== "string") {
    return null;
  }

  try {
    const event = JSON.parse(payload) as Partial<LiveEvent>;
    if (event.type === "position_updated" && isPositionUpdatedEvent(event)) {
      return event;
    }

    if (
      event.type === "position_rejected" ||
      event.type === "message_rejected" ||
      event.type === "connection_established"
    ) {
      return event as LiveEvent;
    }
  } catch {
    return null;
  }

  return null;
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

function positionUpdatePayload(position: GeolocationPosition) {
  const { coords } = position;
  return {
    type: "position_update",
    latitude: coords.latitude,
    longitude: coords.longitude,
    accuracyM: finiteOrUndefined(coords.accuracy),
    altitudeM: finiteOrUndefined(coords.altitude),
    speedMps: finiteOrUndefined(coords.speed),
    headingDeg: finiteOrUndefined(coords.heading),
    clientRecordedAt: new Date(position.timestamp).toISOString(),
  };
}

function finiteOrUndefined(value: number | null): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function positionRejectedMessage(error?: string) {
  if (error === "route_closed") {
    return "This route is closed.";
  }

  if (error === "unauthorized") {
    return "Route access expired. Join again to continue.";
  }

  if (error === "invalid_input") {
    return "The latest location update could not be used.";
  }

  return "The latest location update was rejected.";
}

function compareMembers(first: SnapshotMember, second: SnapshotMember) {
  const statusOrder: Record<string, number> = {
    tracking: 1,
    stale: 2,
    spectating: 3,
    offline: 4,
    left: 5,
  };

  if (first.role !== second.role) {
    return first.role === "owner" ? -1 : 1;
  }

  const firstStatus = statusOrder[first.status] ?? 99;
  const secondStatus = statusOrder[second.status] ?? 99;
  if (firstStatus !== secondStatus) {
    return firstStatus - secondStatus;
  }

  return new Date(first.joinedAt).getTime() - new Date(second.joinedAt).getTime();
}

function formatRole(role: string) {
  return role === "owner" ? "Owner" : "Member";
}

function formatStatus(status: string) {
  return status
    .split("_")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function formatTransportMode(mode: string) {
  return transportLabels[mode as TransportMode] ?? formatStatus(mode);
}

function RouteHeader({
  code,
  label,
  title,
}: {
  code: string;
  label: string;
  title?: string;
}) {
  return (
    <div className="form-header">
      <p className="eyebrow">{label}</p>
      <h1>{title || code}</h1>
      <p className="route-code">{code}</p>
    </div>
  );
}
