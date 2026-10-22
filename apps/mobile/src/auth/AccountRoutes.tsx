import { useEffect, useMemo, useRef, useState } from 'react';
import type { MutableRefObject } from 'react';
import { ActivityIndicator, AppState, BackHandler, Pressable, ScrollView, Share, View } from 'react-native';
import { BrandText as Text } from '../components/Brand';
import { ActionButton, ErrorMessage, Field, Screen, styles } from '../components/ui';
import { TransportSelect } from '../components/TransportSelect';
import { getApiBaseUrl } from '../api/config';
import { parseRouteCode } from '../domain/routes';
import type { Membership, RouteAccess, SharingPolicy, TransportMode } from '../domain/routes';
import { routeShareUrl, shareRouteLink } from '../domain/route-share';
import { RouteSnapshotScreen } from '../screens/RouteSnapshotScreen';
import type { AccountController } from './controller';
import { AccountError } from './api';
import { appendPage, createAccountRoutesApi, freshList, routeError } from './route-api';
import type { AccountRoutesApi, RouteFilters, RouteListCache } from './route-api';

type Page = { kind: 'list' } | { kind: 'create' } | { kind: 'join'; code: string } | { kind: 'route'; member: Membership };
export function AccountRoutes({ controller, cache, onAccount, accountNotice }: {
  controller: AccountController; cache: MutableRefObject<RouteListCache>; onAccount: () => void; accountNotice?: string | null;
}) {
  const api = useMemo(() => createAccountRoutesApi(controller), [controller]);
  const account = useMemo(() => ({ api, revoked: () => { void controller.revoke(); } }), [api, controller]);
  const [page, setPage] = useState<Page>({ kind: 'list' });
  const [notice, setNotice] = useState<string | null>(null);
  const user = controller.snapshot().user!;
  const target = controller.snapshot().destination;
  useEffect(() => {
    let active = true;
    if (target === '/account') { onAccount(); return; }
    if (target === '/routes/new') setPage({ kind: 'create' });
    else if (/^\/routes\/[A-Z0-9]{6}$/.test(target)) {
      const code = target.slice(-6);
      void api.access(code, new AbortController().signal).then(() => { if (active) setPage({ kind: 'join', code }); })
        .catch((error) => { if (active) { setNotice(routeError(error)); setPage({ kind: 'list' }); } });
    }
    return () => { active = false; };
  }, [target, api]);
  function list() { setPage({ kind: 'list' }); void controller.setDestination('/my-routes'); }
  function open(code: string) {
    setNotice(null);
    setPage({ kind: 'route', member: { code, memberId: '', memberToken: '' } });
    // Route membership is server-owned. No legacy profile/token is read or stored.
  }
  if (page.kind === 'route') return <RouteSnapshotScreen key={page.member.code} member={page.member} account={account}
    onChooseRoute={(code, message) => { setNotice(message ?? null); if (code) setPage({ kind: 'join', code }); else list(); }} />;
  if (page.kind === 'create' || page.kind === 'join') return <RouteEntry key={page.kind + (page.kind === 'join' ? page.code : '')}
    api={api} username={user.username} create={page.kind === 'create'} initialCode={page.kind === 'join' ? page.code : ''} onBack={list} onOpen={open} />;
  return <MyRoutes api={api} cache={cache} username={user.username} notice={notice ?? accountNotice ?? null}
    onCreate={() => setPage({ kind: 'create' })} onJoin={() => setPage({ kind: 'join', code: '' })}
    onAccount={onAccount} onOpen={(code) => setPage({ kind: 'join', code })} />;
}

function MyRoutes({ api, cache, username, notice, onCreate, onJoin, onAccount, onOpen }: {
  api: AccountRoutesApi; cache: MutableRefObject<RouteListCache>; username: string; notice: string | null;
  onCreate: () => void; onJoin: () => void; onAccount: () => void; onOpen: (code: string) => void;
}) {
  const [filters, setFilters] = useState<RouteFilters>(cache.current.filters);
  const [result, setResult] = useState(cache.current.result);
  const [busy, setBusy] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [offline, setOffline] = useState(false);
  const [shareNotice, setShareNotice] = useState<string | null>(null);
  const scroll = useRef<ScrollView>(null);
  const request = useRef<AbortController | null>(null);
  const mounted = useRef(true);
  const loading = useRef(false);
  const restore = useRef(cache.current.scroll);
  async function load(more = false) {
    if (loading.current) return;
    loading.current = true; setBusy(true); setError(null);
    const controller = new AbortController(); request.current = controller;
    try {
      let next = await api.list(filters, controller.signal, more ? cache.current.result?.nextCursor : null);
      if (more && cache.current.result) next = appendPage(cache.current.result, next);
      else {
        // Revalidate every retained page on Back, keeping the loaded position.
        for (let index = 1; index < cache.current.pages && next.nextCursor; index++) {
          next = appendPage(next, await api.list(filters, controller.signal, next.nextCursor));
        }
      }
      if (controller.signal.aborted || !mounted.current) return;
      setResult(next); setOffline(false);
      cache.current = { ...cache.current, filters, result: next, pages: more ? cache.current.pages + 1 : cache.current.pages };
    } catch (reason) {
      if (!controller.signal.aborted && mounted.current) {
        setError(routeError(reason)); setOffline(reason instanceof AccountError && ['offline', 'request_cancelled'].includes(reason.code));
      }
    } finally { if (request.current === controller) { loading.current = false; if (mounted.current && !controller.signal.aborted) setBusy(false); } }
  }
  useEffect(() => {
    mounted.current = true; void load();
    const subscription = AppState.addEventListener('change', (state) => { if (state === 'active') void load(); });
    return () => { mounted.current = false; request.current?.abort(); subscription.remove(); };
  }, [filters]);
  function change(next: RouteFilters) {
    request.current?.abort(); loading.current = false;
    cache.current = { ...freshList(), filters: next }; restore.current = 0;
    setResult(null); setFilters(next); setOffline(false); scroll.current?.scrollTo({ y: 0, animated: false });
  }
  const allOff = !filters.owned && !filters.joined && !filters.left;
  return <Screen scrollRef={scroll} onScroll={(y) => { cache.current.scroll = y; }} onContentSizeChange={() => {
    if (restore.current > 0) { scroll.current?.scrollTo({ y: restore.current, animated: false }); restore.current = 0; }
  }}>
    <View style={{ flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 }}>
      <Text style={styles.title} accessibilityRole="header">My routes</Text>
      <Pressable accessibilityRole="button" accessibilityLabel={`Account · ${username}`} onPress={onAccount} style={[styles.button, styles.secondaryButton]}><Text style={styles.label}>Account</Text></Pressable>
    </View>
    <View style={{ flexDirection: 'row', gap: 12 }}>
      <View style={{ flex: 1 }}><ActionButton label="Create a route" disabled={offline || busy} onPress={onCreate} /></View>
      <View style={{ flex: 1 }}><ActionButton label="Join by code or link" secondary disabled={offline || busy} onPress={onJoin} /></View>
    </View>
    {notice && <Text style={styles.text}>{notice}</Text>}
    <View style={{ flexDirection: 'row', gap: 12 }}>{(['active', 'archived'] as const).map((view) => <Pressable key={view}
      accessibilityRole="tab" accessibilityState={{ selected: view === filters.view }} onPress={() => change({ ...filters, view })}
      style={[styles.button, styles.secondaryButton, { flex: 1 }, view === filters.view && { borderColor: '#22c55e' }]}>
      <Text style={styles.label}>{view === 'active' ? 'Active' : 'Archived'} ({result?.counts[view] ?? 0})</Text>
    </Pressable>)}</View>
    <View style={{ flexDirection: 'row', gap: 8 }}>{(['owned', 'joined', 'left'] as const).map((filter) => <Pressable key={filter} accessibilityRole="checkbox"
      accessibilityLabel={({ owned: 'Owned by me', joined: 'Joined routes', left: 'Left routes' })[filter]}
      accessibilityState={{ checked: filters[filter] }} onPress={() => change({ ...filters, [filter]: !filters[filter] })}
      style={[styles.input, { flex: 1, alignItems: 'center', justifyContent: 'center' }, filters[filter] && { borderColor: '#22c55e', backgroundColor: '#173023' }]}>
      <Text style={[styles.label, { fontSize: 14 }]}>{({ owned: 'Owned', joined: 'Joined', left: 'Left' })[filter]}</Text>
    </Pressable>)}</View>
    {busy && <ActivityIndicator color="#22c55e" accessibilityLabel="Loading routes" />}
    <ErrorMessage message={error} />
    {offline && <Text style={styles.text}>Offline. Retained summaries are read-only. You can still share a route link.</Text>}
    {error && <ActionButton label="Retry loading routes" disabled={busy} onPress={() => void load()} />}
    {!busy && !error && !result?.items.length && <Text style={styles.text}>{allOff ? 'Select at least one category to see routes.' : filters.owned && filters.joined && !filters.left ? `No ${filters.view} routes yet. Create or join a route to get started.` : 'No routes match these filters. Try another category or tab.'}</Text>}
    {shareNotice && <Text selectable style={styles.text}>{shareNotice}</Text>}
    {result?.items.map((item) => <View style={styles.card} key={item.id}>
      <Text style={styles.label}>{item.name}</Text><Text selectable style={styles.text}>Code {item.code}</Text>
      <Text style={styles.text}>{({ owner: 'Owner', joined: 'Joined', left: 'Former member' })[item.relationship]} · {item.hasPassword ? 'Password protected' : 'No password'}</Text>
      <Text style={styles.text}>{item.status === 'closed' ? 'Closed' : 'Created'} {new Date(item.closedAt ?? item.createdAt).toLocaleDateString()}</Text>
      <ActionButton label={item.action === 'rejoin' ? item.status === 'closed' ? 'Rejoin to view archive' : 'Rejoin' : item.action === 'view_archive' ? 'View archive' : 'Open route'} disabled={offline || busy} onPress={() => onOpen(item.code)} />
      <ActionButton label={`Share ${item.name}`} secondary onPress={() => {
        const url = routeShareUrl(item.code, getApiBaseUrl(), process.env.EXPO_PUBLIC_AUTH_LINK_ORIGIN);
        void shareRouteLink(url, Share.share).then(setShareNotice);
      }} />
    </View>)}
    {result?.nextCursor && <ActionButton label={busy ? 'Loading…' : 'Load more'} disabled={busy || offline} onPress={() => void load(true)} />}
  </Screen>;
}

function RouteEntry({ api, username, create, initialCode, onBack, onOpen }: {
  api: AccountRoutesApi; username: string; create: boolean; initialCode: string; onBack: () => void; onOpen: (code: string) => void;
}) {
  const [input, setInput] = useState(initialCode);
  const [access, setAccess] = useState<RouteAccess | null>(null);
  const [name, setName] = useState(''); const [description, setDescription] = useState('');
  const [password, setPassword] = useState(''); const [transportMode, setTransport] = useState<TransportMode>('walking');
  const [policy, setPolicy] = useState<SharingPolicy>('everyone_can_share');
  const [busy, setBusy] = useState(false); const [error, setError] = useState<string | null>(null);
  const request = useRef<AbortController | null>(null); const inFlight = useRef(false);
  const mounted = useRef(true);
  useEffect(() => () => { mounted.current = false; request.current?.abort(); }, []);
  useEffect(() => {
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => { if (!inFlight.current) onBack(); return true; });
    return () => subscription.remove();
  }, [onBack]);
  useEffect(() => { if (initialCode && !create) void inspect(); }, []);
  async function inspect() {
    if (inFlight.current) return;
    inFlight.current = true; setBusy(true); setError(null);
    const controller = new AbortController(); request.current = controller;
    try {
      const code = parseRouteCode(input);
      // A current membership restores through GET and never creates another episode.
      try { await api.snapshot(code, controller.signal); if (!controller.signal.aborted) onOpen(code); return; }
      catch (reason) { if (!(reason instanceof AccountError && reason.status === 403)) throw reason; }
      const metadata = await api.access(code, controller.signal);
      if (!controller.signal.aborted) { setInput(code); setAccess(metadata); }
    } catch (reason) { if (!controller.signal.aborted) setError(reason instanceof AccountError ? routeError(reason) : reason instanceof Error ? reason.message : 'Invalid code.'); }
    finally { inFlight.current = false; if (mounted.current) setBusy(false); }
  }
  async function submit() {
    if (inFlight.current || (create && !name.trim())) { if (!name.trim() && create) setError('Enter a route name.'); return; }
    inFlight.current = true; setBusy(true); setError(null);
    const controller = new AbortController(); request.current = controller;
    try {
      let code: string;
      if (create) code = await api.create({ name: name.trim(), description: description.trim(), password, sharingPolicy: policy, transportMode }, controller.signal);
      else { code = parseRouteCode(input); await api.join(code, { password, transportMode }, controller.signal); }
      setPassword(''); if (!controller.signal.aborted) onOpen(code);
    } catch (reason) { if (!controller.signal.aborted) setError(routeError(reason)); }
    finally { inFlight.current = false; if (mounted.current) setBusy(false); }
  }
  return <Screen>
    <Text style={styles.title}>{create ? 'Create a route' : access?.status === 'closed' ? 'Join archive' : 'Join a route'}</Text>
    <Text style={styles.text}>Joining as {username}. Other members see your account username.</Text>
    {create ? <>
      <Field label="Route name" value={name} onChangeText={setName} editable={!busy} />
      <Field label="Description (optional)" value={description} onChangeText={setDescription} multiline editable={!busy} />
    </> : <Field label="Route code or link" value={input} editable={!busy && !access} onChangeText={setInput} autoCapitalize="characters" />}
    {access && <Text style={styles.text}>{access.name} · {access.status === 'closed' ? 'Read-only archive' : 'Active route'}</Text>}
    {(create || access) && <>
      <TransportSelect value={transportMode} onChange={setTransport} disabled={busy} />
      {(create || access?.requiresPassword) && <Field label={create ? 'Route password (optional)' : 'Route password'} value={password} onChangeText={setPassword} secureTextEntry autoCapitalize="none" editable={!busy} />}
      {create && <ActionButton label={policy === 'everyone_can_share' ? 'Everyone can share' : 'Only owner can share'} secondary disabled={busy} onPress={() => setPolicy(policy === 'everyone_can_share' ? 'joiners_can_view_only' : 'everyone_can_share')} />}
    </>}
    <ErrorMessage message={error} />
    <ActionButton label={busy ? 'Please wait…' : create ? 'Create route' : access ? 'Join route' : 'Continue'} disabled={busy} onPress={() => void (create || access ? submit() : inspect())} />
    <Text style={styles.text}>Opening a route never starts location sharing.</Text>
    <ActionButton label="Back to My routes" secondary disabled={busy} onPress={onBack} />
  </Screen>;
}
