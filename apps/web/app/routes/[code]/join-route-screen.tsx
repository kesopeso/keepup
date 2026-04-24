"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  getProfile,
  getRouteAuth,
  saveProfile,
  saveRouteAuth,
  transportModes,
  type TransportMode,
} from "../../../lib/identity-storage";
import {
  getRouteAccess,
  joinRoute,
  type RouteAccess,
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
  const [hasSavedAccess, setHasSavedAccess] = useState(false);

  useEffect(() => {
    let isMounted = true;

    const profile = getProfile();
    setDisplayName(profile.displayName);
    setTransportMode(profile.transportMode);

    const routeAuth = getRouteAuth(code);
    if (routeAuth?.memberToken) {
      setHasSavedAccess(true);
      setIsLoading(false);
      return;
    }

    getRouteAccess(code)
      .then((routeAccess) => {
        if (!isMounted) {
          return;
        }

        setAccess(routeAccess);
        setIsLoading(false);
      })
      .catch((caughtError) => {
        if (!isMounted) {
          return;
        }

        setError(
          caughtError instanceof Error
            ? caughtError.message
            : "Could not load this route.",
        );
        setIsLoading(false);
      });

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
      setHasSavedAccess(true);
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

  if (hasSavedAccess) {
    return (
      <section className="route-shell">
        <RouteHeader code={code} label="Route" />
        <div className="route-panel">
          <p className="route-status">Saved access found.</p>
        </div>
      </section>
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
