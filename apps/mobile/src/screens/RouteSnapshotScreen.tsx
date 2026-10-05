import { useEffect, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { RouteMap } from '../components/RouteMap';
import type { RouteMapRef } from '../components/RouteMap';
import { snapshotGeometry } from '../map/snapshot-geometry';
import { ActionButton, ErrorMessage, Screen, styles } from '../components/ui';
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
  const [attempt, setAttempt] = useState(0);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const map = useRef<RouteMapRef>(null);
  const scroll = useRef<ScrollView>(null);
  const locatedMembers = useMemo(() => new Set(snapshot ? snapshotGeometry(snapshot).markers.features.map((feature) => feature.properties.memberId) : []), [snapshot]);

  useEffect(() => {
    const controller = new AbortController();
    void loadMemberSnapshot(api, repository, member, controller.signal).then((result) => {
      if (!controller.signal.aborted) {
        setSnapshot(result);
        setUpdatedAt(new Date());
      }
    }).catch((error: unknown) => {
      if (!controller.signal.aborted) {
        setError(error instanceof Error ? error.message : 'Could not load the route. Please try again.');
        setInvalid(invalidMembership(error));
        if (invalidMembership(error)) setSnapshot(null);
      }
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [api, repository, member, attempt]);

  function refresh() {
    if (loading) return;
    setLoading(true);
    setError(null);
    setAttempt((previous) => previous + 1);
  }

  return <Screen scrollRef={scroll}>
    <Text style={styles.title} accessibilityRole="header">{snapshot?.route.name ?? `Route ${member.code}`}</Text>
    {loading && <View style={localStyles.row}><ActivityIndicator color="#22c55e" /><Text style={styles.text}>Loading route…</Text></View>}
    <ErrorMessage message={error} />
    {snapshot && <>
      <RouteMap ref={map} snapshot={snapshot} />
      <View style={styles.card}>
        <Text style={styles.label}>{snapshot.route.status === 'closed' ? 'Closed route · Archive' : 'Active route'}</Text>
        <Text style={styles.text}>Code {snapshot.route.code}</Text>
        {!!snapshot.route.description && <Text style={styles.text}>{snapshot.route.description}</Text>}
        {snapshot.route.status === 'active' && <Text style={styles.text}>{snapshot.route.sharingPolicy === 'joiners_can_view_only' ? 'Only the owner may share a location.' : 'All members may share a location.'}</Text>}
        {snapshot.route.status === 'closed' && <Text style={styles.text}>This route is a read-only archive.</Text>}
        <Text style={styles.text}>Last updated {updatedAt?.toLocaleTimeString()}. Refresh to see the latest details and member status.</Text>
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
    <ActionButton label="Join another route" secondary onPress={() => onChooseRoute()} />
  </Screen>;
}

const localStyles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  member: { flexDirection: 'row', alignItems: 'center', gap: 12, minHeight: 48 },
  memberText: { flex: 1, gap: 4 },
  dot: { width: 12, height: 12, borderRadius: 6 },
});
