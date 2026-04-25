"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
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
  type RouteAccess,
  type RouteSnapshot,
  type SnapshotMember,
} from "../../../lib/routes-api";

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
  const [snapshot, setSnapshot] = useState<RouteSnapshot | null>(null);

  useEffect(() => {
    let isMounted = true;

    const profile = getProfile();
    setDisplayName(profile.displayName);
    setTransportMode(profile.transportMode);

    const routeAuth = getRouteAuth(code);
    if (routeAuth?.memberToken) {
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
    return <RouteSnapshotShell snapshot={snapshot} />;
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

function RouteSnapshotShell({ snapshot }: { snapshot: RouteSnapshot }) {
  const sortedMembers = [...snapshot.members].sort(compareMembers);
  const totalPathPoints = snapshot.members.reduce(
    (total, member) =>
      total +
      member.paths.reduce(
        (memberTotal, path) => memberTotal + path.points.length,
        0,
      ),
    0,
  );

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

      <section className="map-stage" aria-label="Route map">
        <div className="map-surface">
          <div className="map-grid" aria-hidden="true" />
          <div className="map-state">
            <strong>{totalPathPoints}</strong>
            <span>{totalPathPoints === 1 ? "point" : "points"}</span>
          </div>
        </div>
      </section>

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
