import { BrandText as Text } from '../components/Brand';
import { useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, AppState, Linking, Platform, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { RouteMap } from '../components/RouteMap';
import type { RouteMapRef } from '../components/RouteMap';
import { snapshotGeometry } from '../map/snapshot-geometry';
import { ActionButton, ErrorMessage, Screen, styles } from '../components/ui';
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
  const scroll = useRef<ScrollView>(null);
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

  return <Screen scrollRef={scroll}>
    <Text style={styles.title} accessibilityRole="header">{snapshot?.route.name ?? `Route ${member.code}`}</Text>
    {loading && <View style={localStyles.row}><ActivityIndicator color="#22c55e" /><Text style={styles.text}>Loading route…</Text></View>}
    <Text style={styles.text} accessibilityLiveRegion="polite">{({
      connecting: 'Connecting to live updates…', live: 'Live updates connected',
      reconnecting: 'Connection interrupted. Reconnecting…', paused: 'Live updates paused',
      archive: 'Archive · Live updates stopped', unavailable: 'Live updates unavailable', invalid: 'Saved access is no longer valid',
    })[liveStatus]}</Text>
    <ErrorMessage message={error} />
    {snapshot && <>
      {snapshot.route.status === 'active' && <View style={styles.card}>
        <Text style={styles.label}>{sharingState.recovery ? 'Continue sharing your location?' : 'Location sharing'}</Text>
        <Text style={styles.text}>{Platform.OS === 'android' ? 'Sharing continues with the screen locked. Allow location and notifications. Tap the KeepUp notification to return here and stop. Sharing uses battery and mobile data.' : 'Keep KeepUp open to share your location. Leaving the app pauses GPS capture.'}</Text>
        <ErrorMessage message={sharingState.error} />
        {sharingState.settings && <ActionButton label="Open app settings" secondary onPress={() => {
          void Linking.openSettings().catch(() => setSharingState((state) => ({ ...state, error: 'Could not open settings. Open KeepUp permissions from Android Settings.' })));
        }} />}
        {(snapshot.viewer.canStartSharing || sharingState.recovery) && !sharingState.sharing && <ActionButton
          label={sharingState.action === 'start' ? 'Finding location…' : sharingState.recovery ? 'Resume sharing' : 'Start sharing location'}
          disabled={liveStatus !== 'live' || !!sharingState.action} onPress={() => { void sharing.current?.start(); }} />}
        {(snapshot.viewer.canStopSharing || sharingState.sharing || sharingState.recovery) && <ActionButton
          label={sharingState.action === 'stop' ? 'Stopping…' : sharingState.recovery ? 'Continue as spectator' : 'Stop sharing location'}
          disabled={!!sharingState.action || (liveStatus !== 'live' && !sharingState.sharing)} onPress={() => { void sharing.current?.stop(); }} />}
        {!snapshot.viewer.canStartSharing && !snapshot.viewer.canStopSharing && <Text style={styles.text}>Location sharing is not available for your membership.</Text>}
        {liveStatus !== 'live' && <Text style={styles.text}>Wait for the live connection to confirm sharing changes.</Text>}
      </View>}
      <RouteMap ref={map} snapshot={snapshot} />
      <View style={styles.card}>
        <Text style={styles.label}>{snapshot.route.status === 'closed' ? 'Closed route · Archive' : 'Active route'}</Text>
        <Text style={styles.text}>Code {snapshot.route.code}</Text>
        {!!snapshot.route.description && <Text style={styles.text}>{snapshot.route.description}</Text>}
        {snapshot.route.status === 'active' && <Text style={styles.text}>{snapshot.route.sharingPolicy === 'joiners_can_view_only' ? 'Only the owner may share a location.' : 'All members may share a location.'}</Text>}
        {snapshot.route.status === 'closed' && <Text style={styles.text}>This route is a read-only archive.</Text>}
        <Text style={styles.text}>Last updated {updatedAt?.toLocaleTimeString()}. {snapshot.route.status === 'active' ? 'Locations update automatically while connected.' : 'Saved archive.'}</Text>
      </View>
      <View style={styles.card}>
        <Text style={styles.title} accessibilityRole="header">Members · {snapshot.members.length}</Text>
        {snapshot.members.map((person) => <Pressable key={person.id} style={localStyles.member}
          accessibilityRole="button" accessibilityLabel={`Show ${person.displayName} on map`}
          accessibilityState={{ disabled: !locatedMembers.has(person.id) }} disabled={!locatedMembers.has(person.id)}
          onPress={() => { map.current?.focusMember(person.id); scroll.current?.scrollTo({ y: 0, animated: false }); }}>
          <View style={[localStyles.dot, { backgroundColor: person.color }]} />
          <View style={localStyles.memberText}>
            <Text style={styles.label}>{person.displayName}{person.id === snapshot.viewer.memberId ? ' · You' : ''}{person.role === 'owner' ? ' · Owner' : ''}</Text>
            <Text style={styles.text}>{transportLabels[person.transportMode]} · {statusLabels[person.status]}</Text>
            <Text style={styles.text}>{locatedMembers.has(person.id) ? 'Show on map' : 'No saved location'}</Text>
          </View>
        </Pressable>)}
        {snapshot.members.length === 0 && <Text style={styles.text}>No members to display.</Text>}
      </View>
    </>}
    {invalid ? <ActionButton label="Join again" onPress={() => onChooseRoute(member.code, error ?? undefined)} /> :
      <ActionButton label={loading ? 'Loading…' : error ? 'Retry' : 'Refresh'} disabled={loading} onPress={refresh} />}
    <ActionButton label="Join another route" secondary disabled={!!sharingState.action || (sharingState.sharing && liveStatus !== 'live')}
      onPress={() => { void (async () => {
        if (sharingState.sharing || sharingState.recovery) {
          if (!await sharing.current?.stop()) return;
        }
        onChooseRoute();
      })(); }} />
  </Screen>;
}

const localStyles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  member: { flexDirection: 'row', alignItems: 'center', gap: 12, minHeight: 48 },
  memberText: { flex: 1, gap: 4 },
  dot: { width: 12, height: 12, borderRadius: 6 },
});
