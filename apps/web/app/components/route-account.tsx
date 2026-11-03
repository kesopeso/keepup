"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { AuthError, authMessage, type AccountSession } from "../../lib/auth-api";
import { currentSession, refreshAccountSession, subscribeSession, watchAccountSession } from "../../lib/account-session";
import { rememberDestination } from "../../lib/auth-destination";

export function RouteAccount({ destination, children }: { destination: string; children: (session: AccountSession) => ReactNode }) {
  const router = useRouter();
  const [session, setSession] = useState<AccountSession | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    const update = () => {
      if (!active) return;
      setError("");
      const session = currentSession();
      if (!session?.user.verified) {
        setSession(null);
        rememberDestination(destination);
        router.replace(`${session ? "/auth/verify" : "/auth/login"}?next=${encodeURIComponent(destination)}`);
      } else setSession(session);
    };
    const failure = (caught: unknown) => {
      if (!active) return;
      setError(authMessage(caught));
    };
    let unsubscribe = () => {};
    let stop = () => {};
    refreshAccountSession().then(() => {
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
  if (error || !session) return <main className="route-form"><p role={error ? "alert" : "status"}>{error || "Checking your account..."}</p>{error ? <button onClick={() => window.location.reload()}>Try again</button> : null}</main>;
  return <div key={session.user.id}>{children(session)}</div>;
}
