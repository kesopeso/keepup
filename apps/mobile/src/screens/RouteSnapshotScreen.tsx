import { BrandHeader, BrandText as Text } from '../components/Brand';
import { useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, AppState, BackHandler, Linking, Platform, Pressable, StatusBar as NativeStatusBar, StyleSheet, View } from 'react-native';
import { StatusBar } from 'expo-status-bar';
import { RouteDrawer } from '../components/RouteDrawer';
import { RouteMap } from '../components/RouteMap';
import type { RouteMapRef } from '../components/RouteMap';
import { snapshotGeometry } from '../map/snapshot-geometry';
import { ActionButton, ErrorMessage, styles } from '../components/ui';
import { createViewingSession, viewingSocketUrl } from '../live/viewing-session';
import type { ViewingStatus } from '../live/viewing-session';
import { createForegroundSharing } from '../location/foreground-sharing';
import type { SharingState } from '../location/foreground-sharing';
import { prepareLocation, requestLocationPermission, watchLocation } from '../location/native-location';
import { watchScreenOffLocation, checkScreenOffLocation } from '../location/screen-off-location';
import { getApiBaseUrl } from '../api/config';
import { invalidMembership } from '../api/routes';
import type { RoutesApi } from '../api/routes';
import { loadMemberSnapshot } from '../domain/join-session';
import { transportLabels } from '../domain/routes';
import type { MemberStatus, Membership, RouteSnapshot } from '../domain/routes';
import type { SessionRepository } from '../storage/session-repository';

const statusLabels: Record<MemberStatus, string> = {
  tracking: 'Sharing location', spectating: 'Spectating', stale: 'Location delayed', offline: 'Offline', left: 'Left',
};

export function RouteSnapshotScreen({ api, repository, member, onChooseRoute }: {
  api: RoutesApi; repository: SessionRepository; member: Membership;
  onChooseRoute: (code?: string, error?: string) => void;
}) {
  const [snapshot, setSnapshot] = useState<RouteSnapshot | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [invalid, setInvalid] = useState(false);
  const [liveStatus, setLiveStatus] = useState<ViewingStatus>('connecting');
  const [sharingState, setSharingState] = useState<SharingState>({ action: null, sharing: false, recovery: false, error: null, settings: false });
  const sharing = useRef<ReturnType<typeof createForegroundSharing> | null>(null);
  const session = useRef<ReturnType<typeof createViewingSession> | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const map = useRef<RouteMapRef>(null);
  const [expanded, setExpanded] = useState(false);
  const [height, setHeight] = useState(0);
  const [headerHeight, setHeaderHeight] = useState(140);
  const [peekHeight, setPeekHeight] = useState(196);
  const bottomInset = Platform.OS === 'android' ? 24 : 34;
  const drawerHeight = Math.max(1, height - headerHeight - 12);

  useEffect(() => {
    if (!expanded) return;
    const back = BackHandler.addEventListener('hardwareBackPress', () => { setExpanded(false); return true; });
    return () => back.remove();
  }, [expanded]);

  useEffect(() => {
    if (sharingState.error || sharingState.settings) setExpanded(true);
  }, [sharingState.error, sharingState.settings]);
  const locatedMembers = useMemo(() => new Set(snapshot ? snapshotGeometry(snapshot).markers.features.map((feature) => feature.properties.memberId) : []), [snapshot]);

  useEffect(() => {
    const android = Platform.OS === 'android';
    let viewing: ReturnType<typeof createViewingSession> | undefined;
    const capture = createForegroundSharing({ keepWatchOnDisconnect: android,
      requestPermission: requestLocationPermission, prepare: prepareLocation, watch: android ? watchScreenOffLocation : watchLocation,
      wake: () => viewing?.wake(),
      command: (type) => viewing!.command(type), send: (payload) => viewing!.sendPosition(payload),
      onState: (state) => { setSharingState(state); viewing?.setBackgroundSharing(android && state.sharing); } });
    sharing.current = capture;
    viewing = createViewingSession({
      url: viewingSocketUrl(getApiBaseUrl()), member,
      load: (signal) => loadMemberSnapshot(api, repository, member, signal),
      onSnapshot: (result) => { capture.update(result); setSnapshot(result); setUpdatedAt(new Date()); setLoading(false); },
      onStatus: (status) => { capture.connection(status); setLiveStatus(status); },
      onLiveEvent: capture.event,
      onError: (message, rejected) => {
        setError(message); setInvalid(rejected); setLoading(false);
        if (rejected) setSnapshot(null);
      },
      isInvalid: invalidMembership,
    });
    session.current = viewing;
    viewing.setForeground(AppState.currentState === 'active');
    const subscription = AppState.addEventListener('change', (state) => {
      viewing!.setForeground(state === 'active');
      if (state === 'active' && android) void checkScreenOffLocation();
    });
    return () => { subscription.remove(); capture.dispose(); sharing.current = null; viewing!.stop(); session.current = null; };
  }, [api, repository, member]);

  function refresh() {
    if (loading) return;
    setLoading(true);
    void session.current?.refresh();
  }

  const connectionLabel = ({
    connecting: 'Connecting to live updates…', live: 'Live updates connected',
    reconnecting: 'Connection interrupted. Reconnecting…', paused: 'Live updates paused',
    archive: 'Archive · Live updates stopped', unavailable: 'Live updates unavailable', invalid: 'Saved access is no longer valid',
  })[liveStatus];

  const sharingActions = snapshot?.route.status === 'active' && <>
    {(snapshot.viewer.canStartSharing || sharingState.recovery) && !sharingState.sharing && <ActionButton
      label={sharingState.action === 'start' ? 'Finding location…' : sharingState.recovery ? 'Resume sharing' : 'Start sharing location'}
      disabled={liveStatus !== 'live' || !!sharingState.action} onPress={() => { void sharing.current?.start(); }} />}
    {(snapshot.viewer.canStopSharing || sharingState.sharing || sharingState.recovery) && <ActionButton
      label={sharingState.action === 'stop' ? 'Stopping…' : sharingState.recovery ? 'Continue as spectator' : 'Stop sharing location'}
      disabled={!!sharingState.action || (liveStatus !== 'live' && !sharingState.sharing)} onPress={() => { void sharing.current?.stop(); }} />}
    {!snapshot.viewer.canStartSharing && !snapshot.viewer.canStopSharing && <Text style={styles.text}>Location sharing is not available for your membership.</Text>}
  </>;

  return <View style={[styles.screen, { overflow: 'hidden' }]} onLayout={({ nativeEvent: { layout } }) => setHeight(layout.height)}>
    <StatusBar style="light" />
    {snapshot && <RouteMap ref={map} snapshot={snapshot} topInset={headerHeight + 12}
      bottomInset={peekHeight} controlsHidden={expanded} />}
    <View pointerEvents="none" style={localStyles.statusBarBackground} />
    <View style={localStyles.header} onLayout={({ nativeEvent: { layout } }) => setHeaderHeight(layout.y + layout.height)}>
      <BrandHeader />
      <Text style={styles.title} numberOfLines={1} accessibilityRole="header">{snapshot?.route.name ?? `Route ${member.code}`}</Text>
      <Text style={localStyles.connection} accessibilityLiveRegion="polite">{connectionLabel}</Text>
    </View>
    {height > 0 && <RouteDrawer expanded={expanded} onExpandedChange={setExpanded} height={drawerHeight}
      bottomInset={bottomInset} onPeekHeight={setPeekHeight} summary={<>
        {loading && <View style={localStyles.row}><ActivityIndicator color="#22c55e" /><Text style={styles.text}>Loading route…</Text></View>}
        {snapshot && <Text style={styles.label}>{snapshot.route.status === 'closed' ? 'Closed route · Archive'
          : sharingState.recovery ? 'Continue sharing your location?' : sharingState.sharing ? 'Sharing your location' : 'Viewing as spectator'} · {snapshot.members.length} members</Text>}
        {sharingActions}
        {snapshot?.route.status === 'active' && !sharingState.sharing && !sharingState.recovery &&
          <Text style={localStyles.hint}>{Platform.OS === 'android' ? 'Allow location and notifications. Sharing continues with the screen locked and uses battery and mobile data.' : 'Keep keepup open to share. Sharing uses battery and mobile data.'}</Text>}
        {snapshot?.route.status === 'active' && liveStatus !== 'live' && <Text style={localStyles.hint}>Wait for the live connection to confirm sharing changes.</Text>}
        {!!sharingState.error && !expanded && <ActionButton label="Location needs attention · Open details" secondary onPress={() => setExpanded(true)} />}
        {!!error && !expanded && <ActionButton label="Connection needs attention · Open details" secondary onPress={() => setExpanded(true)} />}
        {!snapshot && <>
          <ErrorMessage message={error} />
          {invalid ? <ActionButton label="Join again" onPress={() => onChooseRoute(member.code, error ?? undefined)} /> :
            <ActionButton label={loading ? 'Loading…' : 'Retry'} disabled={loading} onPress={refresh} />}
        </>}
      </>}>
      <ErrorMessage message={error} />
      {snapshot && <>
        {snapshot.route.status === 'active' && (sharingState.error || sharingState.settings) && <View style={styles.card}>
          <Text style={styles.label}>Location sharing</Text>
          <Text style={styles.text}>{Platform.OS === 'android' ? 'Allow location and notifications. Tap the keepup notification to return here and stop. Sharing continues with the screen locked and uses battery and mobile data.' : 'Keep keepup open to share your location. Leaving the app pauses GPS capture.'}</Text>
          <ErrorMessage message={sharingState.error} />
          {sharingState.settings && <ActionButton label="Open app settings" secondary onPress={() => {
            void Linking.openSettings().catch(() => setSharingState((state) => ({ ...state, error: 'Could not open settings. Open keepup permissions from Android Settings.' })));
          }} />}
        </View>}
        <View style={styles.card}>
          <Text style={styles.title} accessibilityRole="header">Members · {snapshot.members.length}</Text>
          {snapshot.members.map((person) => <Pressable key={person.id} style={localStyles.member}
            accessibilityRole="button" accessibilityLabel={`Show ${person.displayName} on map`}
            accessibilityState={{ disabled: !locatedMembers.has(person.id) }} disabled={!locatedMembers.has(person.id)}
            onPress={() => { setExpanded(false); map.current?.focusMember(person.id); }}>
            <View style={[localStyles.dot, { backgroundColor: person.color }]} />
            <View style={localStyles.memberText}>
              <Text style={styles.label}>{person.displayName}{person.id === snapshot.viewer.memberId ? ' · You' : ''}{person.role === 'owner' ? ' · Owner' : ''}</Text>
              <Text style={styles.text}>{transportLabels[person.transportMode]} · {statusLabels[person.status]}</Text>
              <Text style={styles.text}>{locatedMembers.has(person.id) ? 'Show on map' : 'No saved location'}</Text>
            </View>
          </Pressable>)}
          {snapshot.members.length === 0 && <Text style={styles.text}>No members to display.</Text>}
        </View>
        <View style={styles.card}>
          <Text style={styles.label}>{snapshot.route.status === 'closed' ? 'Closed route · Archive' : 'Active route'}</Text>
          <Text style={styles.label}>{snapshot.route.name}</Text>
          <Text style={styles.text} selectable>Code {snapshot.route.code}</Text>
          {!!snapshot.route.description && <Text style={styles.text}>{snapshot.route.description}</Text>}
          {snapshot.route.status === 'active' && <Text style={styles.text}>{snapshot.route.sharingPolicy === 'joiners_can_view_only' ? 'Only the owner may share a location.' : 'All members may share a location.'}</Text>}
          {snapshot.route.status === 'closed' && <Text style={styles.text}>This route is a read-only archive.</Text>}
          {snapshot.route.status === 'active' && <Text style={styles.text}>{Platform.OS === 'android' ? 'Tap the keepup notification to return here and stop sharing.' : 'Leaving the app pauses GPS capture.'}</Text>}
          <Text style={styles.text}>Last updated {updatedAt?.toLocaleTimeString()}. {snapshot.route.status === 'active' ? 'Locations update automatically while connected.' : 'Saved archive.'}</Text>
        </View>
      </>}
      {snapshot && (invalid ? <ActionButton label="Join again" onPress={() => onChooseRoute(member.code, error ?? undefined)} /> :
        <ActionButton label={loading ? 'Loading…' : error ? 'Retry' : 'Refresh'} disabled={loading} onPress={refresh} />)}
      <ActionButton label="Join another route" secondary disabled={!!sharingState.action || (sharingState.sharing && liveStatus !== 'live')}
        onPress={() => { void (async () => {
          if (sharingState.sharing || sharingState.recovery) {
            if (!await sharing.current?.stop()) return;
          }
          onChooseRoute();
        })(); }} />
    </RouteDrawer>}
  </View>;
}

const localStyles = StyleSheet.create({
  statusBarBackground: { position: 'absolute', top: 0, left: 0, right: 0, height: Platform.OS === 'android' ? NativeStatusBar.currentHeight ?? 0 : 44, backgroundColor: '#0c1014' },
  header: { position: 'absolute', top: (Platform.OS === 'android' ? NativeStatusBar.currentHeight ?? 0 : 44) + 12,
    left: 16, right: 16, padding: 12, gap: 6, backgroundColor: '#0c1014', borderColor: '#34404a', borderWidth: 1, borderRadius: 16 },
  connection: { ...styles.text, fontSize: 13, lineHeight: 18 },
  hint: { ...styles.text, fontSize: 13, lineHeight: 18 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  member: { flexDirection: 'row', alignItems: 'center', gap: 12, minHeight: 48 },
  memberText: { flex: 1, gap: 4 },
  dot: { width: 12, height: 12, borderRadius: 6 },
});
