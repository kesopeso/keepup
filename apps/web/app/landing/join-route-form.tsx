"use client";

import { useId, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import styles from "./landing.module.css";

export function JoinRouteForm() {
  const router = useRouter();
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | null>(null);

  function joinRoute(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const code = String(new FormData(event.currentTarget).get("code") ?? "")
      .trim()
      .toUpperCase();

    if (!code || !/^[A-Z0-9]+$/.test(code)) {
      setError(
        code
          ? "Use the letters and numbers from your invitation."
          : "Enter the route code from your invitation.",
      );
      input.current?.focus();
      return;
    }

    setError(null);
    // The existing route screen checks access and handles missing routes.
    router.push(`/routes/${encodeURIComponent(code)}`);
  }

  return (
    <form className={styles.joinDemo} onSubmit={joinRoute} noValidate>
      <label htmlFor={`${id}-code`}>Enter a route code</label>
      <div className={styles.joinRow}>
        <input
          ref={input}
          id={`${id}-code`}
          name="code"
          type="text"
          placeholder="e.g. K7P4MX"
          autoComplete="off"
          autoCapitalize="characters"
          spellCheck={false}
          maxLength={64}
          required
          aria-invalid={Boolean(error)}
          aria-describedby={`${id}-help${error ? ` ${id}-error` : ""}`}
          onChange={() => error && setError(null)}
        />
        <button className={styles.secondary} type="submit">
          Join <span aria-hidden="true">↗</span>
        </button>
      </div>
      <p id={`${id}-help`} className={styles.joinHelp}>
        Use the code from your route invitation.
      </p>
      {error && (
        <p id={`${id}-error`} className={styles.joinError} role="alert">
          {error}
        </p>
      )}
    </form>
  );
}
