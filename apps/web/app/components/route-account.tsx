"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { AuthError, authMessage, type AccountSession } from "../../lib/auth-api";
import { currentSession, refreshAccountSession, subscribeSession, watchAccountSession } from "../../lib/account-session";
import { rememberDestination } from "../../lib/auth-destination";

// Only the server's explicit account-disabled response permits legacy route authority.
export function RouteAccount({ destination, children }: { destination: string; children: (session: AccountSession | null) => ReactNode }) {
  const router = useRouter();
  const [state, setState] = useState<{ ready: boolean; legacy: boolean; session: AccountSession | null }>({ ready: false, legacy: false, session: null });
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    const update = () => {
      if (!active) return;
      const session = currentSession();
      if (!session?.user.verified) {
        setState({ ready: false, legacy: false, session: null });
        rememberDestination(destination);
        router.replace(`${session ? "/auth/verify" : "/auth/login"}?next=${encodeURIComponent(destination)}`);
      } else setState({ ready: true, legacy: false, session });
    };
    const failure = (caught: unknown) => {
      if (!active) return;
      if (caught instanceof AuthError && caught.code === "auth_unavailable") setState({ ready: true, legacy: true, session: null });
      else setError(authMessage(caught));
    };
    let unsubscribe = () => {};
    let stop = () => {};
    refreshAccountSession(true).then(() => {
      if (!active) return;
      update();
      unsubscribe = subscribeSession(update);
      stop = watchAccountSession(failure);
    }).catch((caught) => {
      if (caught instanceof AuthError && caught.code === "session_expired") update();
      else failure(caught);
    });
    return () => { active = false; unsubscribe(); stop(); };
  }, [destination, router]);
  if (!state.ready) return <main className="route-form"><p role={error ? "alert" : "status"}>{error || "Checking your account..."}</p>{error ? <button onClick={() => window.location.reload()}>Try again</button> : null}</main>;
  return <div key={state.session?.user.id ?? "legacy"}>{children(state.legacy ? null : state.session)}</div>;
}
