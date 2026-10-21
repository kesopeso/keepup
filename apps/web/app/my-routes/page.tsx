"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { AccountSession } from "../../lib/auth-api";
import { accountRouteRequest } from "../../lib/routes-api";
import { appendRoutePage, defaultRouteFilters, readRouteListState, routeListQuery, saveRouteListState, markRouteListReturn, consumeRouteListReturn, isRouteListReturn, type RouteFilters, type RouteListResult } from "../../lib/my-routes-state";
import { shareRouteLink } from "../../lib/route-share";
import { currentSession } from "../../lib/account-session";
import { RouteAccount } from "../components/route-account";
import { AccountMenu } from "../components/account-menu";
import { Brand, NotificationStack, type NotificationItem } from "../components/ui";
import styles from "./routes.module.css";

export default function MyRoutes() {
  return <RouteAccount destination="/my-routes">{session => session ? <RouteList session={session} /> : <main className="route-form"><Brand /><h1>My routes</h1><p>Account sign-in is not enabled on this server yet.</p><a href="/routes/new">Create a route</a></main>}</RouteAccount>;
}

function RouteList({ session }: { session: AccountSession }) {
  const router = useRouter();
  const [saved] = useState(() => isRouteListReturn() || !navigator.onLine ? readRouteListState(session.user.id) : null);
  const [filters, setFilters] = useState<RouteFilters>(saved?.filters ?? defaultRouteFilters);
  const [result, setResult] = useState<RouteListResult | null>(saved?.result ?? null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [offline, setOffline] = useState(false);
  const [hasAnyRoutes, setHasAnyRoutes] = useState<boolean | null>(null);
  const [joinCode, setJoinCode] = useState("");
  const [showJoin, setShowJoin] = useState(false);
  const [manual, setManual] = useState("");
  const [notices, setNotices] = useState<NotificationItem[]>([]);
  const sequence = useRef(0);
  const active = useRef(true);
  const openingRoute = useRef(false);
  const state = useRef({ filters, result });
  state.current = { filters, result };

  async function load(more = false, pages = 1) {
    const request = ++sequence.current;
    setBusy(true); setError("");
    try {
      let page = await accountRouteRequest<RouteListResult>(`/me/routes?${routeListQuery(filters, more ? result?.nextCursor : null)}`);
      for (let index = 1; index < pages && page.nextCursor; index++) {
        if (!active.current || sequence.current !== request) return;
        const next = await accountRouteRequest<RouteListResult>(`/me/routes?${routeListQuery(filters, page.nextCursor)}`);
        page = appendRoutePage(page, next);
      }
      if (!active.current || sequence.current !== request) return;
      setResult(previous => more && previous ? appendRoutePage(previous, page) : page);
      setOffline(false);
      if (page.counts.active + page.counts.archived > 0) setHasAnyRoutes(true);
      else if (filters.owned || filters.joined || filters.left) {
        const all = await accountRouteRequest<RouteListResult>(`/me/routes?${routeListQuery({ ...filters, owned: true, joined: true, left: true })}`);
        if (active.current && sequence.current === request) setHasAnyRoutes(all.counts.active + all.counts.archived > 0);
      }
    } catch (error) {
      if (!active.current || sequence.current !== request) return;
      setOffline(!navigator.onLine || (error instanceof Error && error.message.includes("Cannot reach")));
      setError(error instanceof Error ? error.message : "Could not load your routes.");
    } finally { if (active.current && sequence.current === request) setBusy(false); }
  }

  useEffect(() => {
    active.current = true;
    // The retained page has already been authorized by RouteAccount. Recheck each
    // open, and preserve loaded pagination when returning from a route.
    consumeRouteListReturn();
    if (saved?.result && JSON.stringify(filters) === JSON.stringify(saved.filters)) {
      requestAnimationFrame(() => requestAnimationFrame(() => window.scrollTo(0, saved.scroll)));
      setOffline(!navigator.onLine);
      void load(false, Math.max(1, Math.ceil(saved.result.items.length / 20)));
    } else void load();
    const online = () => { setOffline(false); void load(); };
    const offline = () => setOffline(true);
    window.addEventListener("online", online); window.addEventListener("offline", offline);
    return () => {
      active.current = false; sequence.current++;
      // Never rewrite state after logout has already cleared it.
      if (!openingRoute.current && currentSession()?.user.id === session.user.id) saveRouteListState({ userId: session.user.id, ...state.current, scroll: window.scrollY });
      window.removeEventListener("online", online); window.removeEventListener("offline", offline);
    };
  }, [filters]);
  useEffect(() => {
    const remember = () => { if (!openingRoute.current && currentSession()?.user.id === session.user.id) saveRouteListState({ userId: session.user.id, ...state.current, scroll: window.scrollY }); };
    window.addEventListener("pagehide", remember);
    return () => window.removeEventListener("pagehide", remember);
  }, [session.user.id]);
  useEffect(() => {
    const timers = notices.map(notice => window.setTimeout(() => setNotices(current => current.filter(item => item.id !== notice.id)), 6000));
    return () => timers.forEach(clearTimeout);
  }, [notices]);

  function change(next: RouteFilters) { sequence.current++; setResult(null); setFilters(next); }
  function rememberViewForRoute() {
    saveRouteListState({ userId: session.user.id, filters, result, scroll: window.scrollY });
    openingRoute.current = true;
    markRouteListReturn();
  }
  const hasFilters = filters.owned || filters.joined || filters.left;
  const count = result?.counts[filters.view] ?? 0;
  return <main className={styles.page}>
    <header className={styles.header}><Brand /><AccountMenu username={session.user.username} /></header>
    <div className={styles.content}>
      <div className={styles.title}><div><h1>My routes</h1><p>Pick up where you left off.</p></div><div className={styles.actions}><button disabled={offline} onClick={() => setShowJoin(!showJoin)}>Join route</button><a className="primary-action" href="/routes/new" aria-disabled={offline} onClick={event => { if (offline) { event.preventDefault(); return; } rememberViewForRoute(); }}>+ Create route</a></div></div>
      {showJoin ? <form className={styles.join} onSubmit={event => { event.preventDefault(); if (offline) return; if (/^[A-Za-z0-9]{6}$/.test(joinCode.trim())) { rememberViewForRoute(); router.push(`/routes/${joinCode.trim().toUpperCase()}`); } }}><label>Route code<input value={joinCode} onChange={event => setJoinCode(event.target.value)} autoComplete="off" pattern="[A-Za-z0-9]{6}" required maxLength={6} /></label><button disabled={offline} type="submit">Continue</button></form> : null}
      <div className={styles.tabs} role="tablist" aria-label="Route lifecycle">{(["active", "archived"] as const).map(view => <button key={view} id={`tab-${view}`} role="tab" aria-selected={filters.view === view} aria-controls="route-results" onClick={() => change({ ...filters, view })}>{view === "active" ? "Active" : "Archived"} {result ? `(${result.counts[view]})` : ""}</button>)}</div>
      <fieldset className={styles.filters}><legend className={styles.srOnly}>Route categories</legend>{([['owned', 'My routes'], ['joined', 'Joined routes'], ['left', 'Left routes']] as const).map(([key, label]) => <label key={key}><input type="checkbox" checked={filters[key]} onChange={event => change({ ...filters, [key]: event.target.checked })} />{label}</label>)}</fieldset>
      <section id="route-results" role="tabpanel" aria-labelledby={`tab-${filters.view}`} aria-busy={busy}>
        {offline ? <p className={styles.notice} role="status">You are offline. These are saved summaries; reconnect to open or join a route.</p> : null}
        {error ? <div role="alert"><p>{error}</p><button disabled={busy} onClick={() => load()}>Try again</button></div> : null}
        {busy ? <p role="status">Loading your routes...</p> : null}
        {result?.items.length ? <><div className={styles.listTop}><span>{count} {count === 1 ? "route" : "routes"}</span><span>{filters.view === "active" ? "Newest created first" : "Newest closed first"}</span></div>{result.items.map(item => <article className={styles.route} key={item.id}>
          <div><div className={styles.routeTitle}><h2>{item.name}</h2><span className={styles.badge}>{item.relationship === "owner" ? "Owner" : item.relationship === "left" ? "Left" : "Joined"}</span></div><div className={styles.meta}><span>{item.code}</span><span>{item.hasPassword ? "Password protected" : "Open by link"}</span><span>{item.status === "active" ? "Created" : "Closed"} {new Date(item.closedAt ?? item.createdAt).toLocaleDateString()}</span></div></div>
          <div className={styles.actions}><button disabled={offline} onClick={() => { rememberViewForRoute(); router.push(`/routes/${item.code}`); }}>{item.status === "closed" ? "View archive" : item.action === "rejoin" ? "Rejoin" : "Open route"} ↗</button><button aria-label={`Share ${item.name}`} onClick={async () => { setManual(""); const share = await shareRouteLink(item.code, item.name); if (share && active.current) { setManual(share.manualUrl ?? ""); setNotices(current => [{ id: Date.now(), title: share.title, detail: share.detail, tone: share.tone, leaving: false }, ...current].slice(0, 3)); } }}>Share</button></div>
        </article>)}{result.nextCursor ? <button disabled={busy || offline} onClick={() => load(true)}>Load more</button> : null}</> : !busy && !error ? <div className={styles.empty}><h2>{!hasFilters ? "Choose which routes to show" : hasAnyRoutes === false ? "Your next route starts here" : "No routes match these filters"}</h2><p>{!hasFilters ? "Select My routes, Joined routes, or Left routes." : "Create a route, join with an invitation code, or try another category."}</p><button onClick={() => change({ ...defaultRouteFilters })}>Reset filters</button></div> : null}
      </section>
      {manual ? <label className={styles.manual}>Route link<input readOnly value={manual} onFocus={event => event.target.select()} /></label> : null}
    </div><NotificationStack items={notices} />
  </main>;
}
