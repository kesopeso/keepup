"use client";

import { useEffect, useState } from "react";
import { getRouteAuth } from "../../../lib/identity-storage";

type AuthStatus = "checking" | "ready" | "missing";

export function RouteAuthStatus({ code }: { code: string }) {
  const [status, setStatus] = useState<AuthStatus>("checking");

  useEffect(() => {
    setStatus(getRouteAuth(code)?.memberToken ? "ready" : "missing");
  }, [code]);

  if (status === "checking") {
    return <p className="route-status">Checking saved access...</p>;
  }

  if (status === "ready") {
    return <p className="route-status">Saved access found.</p>;
  }

  return <p className="route-status">No saved access for this browser.</p>;
}
