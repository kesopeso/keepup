"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  getProfile,
  saveProfile,
  saveRouteAuth,
  transportModes,
  type TransportMode,
} from "../lib/identity-storage";
import { createRoute, type SharingPolicy } from "../lib/routes-api";

const sharingPolicyOptions: Array<{
  value: SharingPolicy;
  label: string;
  description: string;
}> = [
  {
    value: "everyone_can_share",
    label: "Everyone can share",
    description: "Joined members may start tracking when slots are available.",
  },
  {
    value: "joiners_can_view_only",
    label: "Joiners view only",
    description: "Only the owner can choose to share location.",
  },
];

const transportLabels: Record<TransportMode, string> = {
  walking: "Walking",
  bicycle: "Bicycle",
  car: "Car",
  bus: "Bus",
  train: "Train",
  boat: "Boat",
  airplane: "Airplane",
};

export function CreateRouteForm() {
  const router = useRouter();
  const [displayName, setDisplayName] = useState("");
  const [transportMode, setTransportMode] = useState<TransportMode>("car");
  const [routeName, setRouteName] = useState("");
  const [description, setDescription] = useState("");
  const [password, setPassword] = useState("");
  const [sharingPolicy, setSharingPolicy] =
    useState<SharingPolicy>("everyone_can_share");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    const profile = getProfile();
    setDisplayName(profile.displayName);
    setTransportMode(profile.transportMode);
  }, []);

  const canSubmit = useMemo(
    () => displayName.trim() !== "" && routeName.trim() !== "" && !isSubmitting,
    [displayName, isSubmitting, routeName],
  );

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");

    if (!canSubmit) {
      setError("Route name and your name are required.");
      return;
    }

    setIsSubmitting(true);

    try {
      const profile = saveProfile({
        displayName,
        transportMode,
      });

      const result = await createRoute({
        clientId: profile.clientId,
        displayName: profile.displayName,
        transportMode: profile.transportMode,
        name: routeName,
        description,
        password,
        sharingPolicy,
      });

      saveRouteAuth(result.route.code, {
        memberToken: result.memberToken,
        ownerToken: result.ownerToken,
      });

      router.push(`/routes/${result.route.code}`);
    } catch (caughtError) {
      setError(
        caughtError instanceof Error
          ? caughtError.message
          : "Could not create the route.",
      );
      setIsSubmitting(false);
    }
  }

  return (
    <form className="route-form" onSubmit={handleSubmit}>
      <div className="form-header">
        <p className="eyebrow">KeepUp</p>
        <h1>Create a route</h1>
      </div>

      <div className="field-grid">
        <label className="field">
          <span>Route name</span>
          <input
            autoComplete="off"
            name="routeName"
            onChange={(event) => setRouteName(event.target.value)}
            placeholder="Morning convoy"
            required
            value={routeName}
          />
        </label>

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
      </div>

      <label className="field">
        <span>Description</span>
        <textarea
          name="description"
          onChange={(event) => setDescription(event.target.value)}
          placeholder="Optional"
          rows={3}
          value={description}
        />
      </label>

      <div className="field-grid">
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

        <label className="field">
          <span>Password</span>
          <input
            autoComplete="new-password"
            name="password"
            onChange={(event) => setPassword(event.target.value)}
            placeholder="Optional"
            type="password"
            value={password}
          />
        </label>
      </div>

      <fieldset className="policy-group">
        <legend>Sharing</legend>
        <div className="policy-options">
          {sharingPolicyOptions.map((option) => (
            <label className="policy-option" key={option.value}>
              <input
                checked={sharingPolicy === option.value}
                name="sharingPolicy"
                onChange={() => setSharingPolicy(option.value)}
                type="radio"
                value={option.value}
              />
              <span>
                <strong>{option.label}</strong>
                <small>{option.description}</small>
              </span>
            </label>
          ))}
        </div>
      </fieldset>

      {error ? (
        <p className="form-error" role="alert">
          {error}
        </p>
      ) : null}

      <button className="primary-action" disabled={!canSubmit} type="submit">
        {isSubmitting ? "Creating..." : "Create route"}
      </button>
    </form>
  );
}
