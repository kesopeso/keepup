"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import {
  getProfile,
  saveProfile,
  saveRouteAuth,
  type TransportMode,
} from "../lib/identity-storage";
import { createRoute, type SharingPolicy } from "../lib/routes-api";
import { Brand, TransportField } from "./components/ui";

const sharingPolicyOptions: Array<{
  value: SharingPolicy;
  label: string;
  description: string;
}> = [
  {
    value: "everyone_can_share",
    label: "Everyone can share",
    description: "Members choose when to share their location.",
  },
  {
    value: "joiners_can_view_only",
    label: "Only the owner can share",
    description: "Only the owner can choose to share location.",
  },
];

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
    <form
      className="route-form"
      onSubmit={handleSubmit}
      aria-busy={isSubmitting}
    >
      <Brand />
      <div className="form-header">
        <p className="eyebrow">Live route sharing</p>
        <h1>Start a route together.</h1>
        <p className="route-description">
          Create a route, share the link, and see your group on the map.
        </p>
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

      <TransportField value={transportMode} onChange={setTransportMode} />

      <details className="route-settings">
        <summary>
          Route settings{" "}
          <span>
            {sharingPolicy === "everyone_can_share"
              ? "Everyone can share"
              : "Only the owner can share"}
            {password ? " · Password protected" : ""}
          </span>
        </summary>
        <div className="settings-fields">
          <label className="field">
            <span>
              Description <small>Optional</small>
            </span>
            <textarea
              name="description"
              onChange={(event) => setDescription(event.target.value)}
              placeholder="Where is your group headed?"
              rows={3}
              value={description}
            />
          </label>
          <label className="field">
            <span>
              Password <small>Optional</small>
            </span>
            <input
              autoComplete="new-password"
              name="password"
              onChange={(event) => setPassword(event.target.value)}
              placeholder="Protect access to this route"
              type="password"
              value={password}
            />
          </label>
          <fieldset className="policy-group">
            <legend>Who can share location?</legend>
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
        </div>
      </details>

      {error ? (
        <p className="form-error" role="alert">
          {error}
        </p>
      ) : null}

      <button className="primary-action" disabled={!canSubmit} type="submit">
        {isSubmitting ? "Creating..." : "Create route"}
      </button>
      <p className="privacy-note">
        Your location stays private until you start sharing.
      </p>
    </form>
  );
}
