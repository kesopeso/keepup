"use client";
import { useState } from "react";
import { authRequest, authMessage } from "../../lib/auth-api";
import { setAccountSession } from "../../lib/account-session";

export function AccountMenu({ username }: { username: string }) {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return <div><details><summary aria-label={`Account menu for ${username}`}>{username}</summary><nav aria-label="Account"><a href="/my-routes">My routes</a><a href="/account">Account settings</a><button disabled={busy} onClick={async () => {
    setBusy(true); setError("");
    try { await authRequest("logout", {}); setAccountSession(null); window.location.assign("/auth/login"); }
    catch (error) { setError(authMessage(error)); setBusy(false); }
  }}>Log out</button></nav></details>{error ? <p role="alert">{error}</p> : null}</div>;
}
