import { useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react';
import type { Ref } from 'react';
import type { LayoutChangeEvent } from 'react-native';
import { AccessibilityInfo, ActivityIndicator, Animated, Linking, Pressable, StyleSheet, Text, View } from 'react-native';
import { Camera, GeoJSONSource, Layer, LogManager, Map, Marker, TransformRequestManager } from '@maplibre/maplibre-react-native';
import type { CameraRef, MapRef } from '@maplibre/maplibre-react-native';
import { ActionButton, ErrorMessage, styles } from './ui';
import type { RouteSnapshot } from '../domain/routes';
import { boundsForPoints, snapshotGeometry } from '../map/snapshot-geometry';
import type { Bounds } from '../map/snapshot-geometry';
import { memberLabelOffset, memberLocationStatus, visibleMemberLabels } from '../map/member-labels';
import { routeMapStyle } from '../map/tile-provider';

export type RouteMapRef = { focusMember: (id: string) => void };
const presence = { tracking: 'Sharing location', spectating: 'Spectating', stale: 'Location delayed', offline: 'Offline', left: 'Left' };

export function RouteMap({ snapshot, ref, topInset, bottomInset, controlsHidden }: {
  snapshot: RouteSnapshot; ref?: Ref<RouteMapRef>; topInset: number; bottomInset: number; controlsHidden: boolean;
}) {
  const geometry = useMemo(() => snapshotGeometry(snapshot), [snapshot]);
  const camera = useRef<CameraRef>(null);
  const mapRef = useRef<MapRef>(null);
  const labelRequest = useRef(0);
  const [labelSizes, setLabelSizes] = useState<Record<string, { width: number; height: number }>>({});
  const [visibleLabels, setVisibleLabels] = useState<string[]>([]);
  const [mapSize, setMapSize] = useState({ width: 0, height: 0 });
  const [ready, setReady] = useState(false);
  const [rendered, setRendered] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [automatic, setAutomatic] = useState(true);
  const automaticRef = useRef(automatic);
  automaticRef.current = automatic;
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [reduceMotion, setReduceMotion] = useState(true);
  const [size, setSize] = useState('');
  const opacity = useRef(new Animated.Value(0)).current;
  const selected = geometry.members.find((member) => member.id === selectedId);
  const locatedMembers = geometry.members.filter((member) => member.latest);

  const updateLabels = useCallback(async () => {
    if (!ready || !mapRef.current) return;
    const request = ++labelRequest.current;
    try {
      const labels = await Promise.all(geometry.members.filter((member) => member.latest).map(async (member) => {
        const [x, y] = await mapRef.current!.project([member.latest!.longitude, member.latest!.latitude]);
        const measured = labelSizes[member.id] ?? { width: 152, height: member.status === 'tracking' ? 30 : 44 };
        return { id: member.id, x, y, ...measured };
      }));
      if (request !== labelRequest.current) return;
      const visible = visibleMemberLabels(labels, { ...mapSize, top: topInset + (selectedId ? 112 : 0), bottom: bottomInset + 48 }, selectedId);
      setVisibleLabels((current) => current.join('|') === visible.join('|') ? current : visible);
    } catch {
      // A map being recreated can reject projection; its next ready event retries.
    }
  }, [ready, geometry, labelSizes, mapSize, topInset, bottomInset, selectedId]);
  useEffect(() => {
    void updateLabels();
    return () => { labelRequest.current++; };
  }, [updateLabels]);

  useEffect(() => {
    let mounted = true;
    void AccessibilityInfo.isReduceMotionEnabled().then((value) => { if (mounted) setReduceMotion(value); });
    const subscription = AccessibilityInfo.addEventListener('reduceMotionChanged', setReduceMotion);
    const header = TransformRequestManager.addHeader({ id: 'keepup-osm-user-agent',
      match: '^https://tile\\.openstreetmap\\.org/', name: 'User-Agent', value: 'KeepUp/1.0 (eu.kesopeso.keepup)' });
    // Native tile failures do not trigger onDidFailLoadingMap, which covers style loading.
    // KeepUp mounts one map at a time, so this handler belongs to this screen's lifecycle.
    LogManager.onLog(({ level, message }) => {
      if (level === 'error' && /Failed to load (tile|source|style)/i.test(message)) {
        if (mounted) setError('Could not load the map tiles. Check your connection and retry.');
        return true;
      }
      return false;
    });
    return () => {
      mounted = false;
      subscription.remove();
      TransformRequestManager.removeHeader(header);
      LogManager.onLog(() => false);
    };
  }, []);

  useEffect(() => {
    if (rendered || error) return;
    const timer = setTimeout(() => setError('The map took too long to load. Check your connection and retry.'), 15_000);
    return () => clearTimeout(timer);
  }, [rendered, error, attempt]);

  const fit = useCallback((bounds: Bounds | null, animate: boolean) => {
    if (!bounds || !camera.current || !ready) return;
    const [west, south, east, north] = bounds;
    const options = { duration: animate && !reduceMotion ? 350 : 0, bearing: 0, pitch: 0,
      padding: { top: topInset + 80, right: 90, bottom: bottomInset + 64, left: 90 } };
    const stop = west === east && south === north
      ? { ...options, center: [west, south] as [number, number], zoom: 15 }
      : { ...options, bounds };
    void camera.current.setStop(stop).catch(() => setError('Could not position the map. Please retry.'));
  }, [ready, reduceMotion, topInset, bottomInset]);

  // Refresh and size changes only move the camera while automatic fitting is enabled.
  useEffect(() => { if (automaticRef.current) fit(geometry.bounds, false); }, [geometry, fit, size]);
  const showFit = ready && !error && !automatic && !!geometry.bounds && !controlsHidden;
  useEffect(() => {
    const animation = Animated.timing(opacity, { toValue: showFit ? 1 : 0, duration: reduceMotion ? 0 : 180, useNativeDriver: true });
    animation.start();
    return () => animation.stop();
  }, [showFit, reduceMotion, opacity]);

  function focusMember(id: string) {
    const person = geometry.members.find((member) => member.id === id);
    if (!person?.latest || !ready || error) return;
    setSelectedId(id);
    automaticRef.current = false;
    setAutomatic(false);
    fit(boundsForPoints(person.points), true);
  }
  useImperativeHandle(ref, () => ({ focusMember }));

  return <View collapsable={false} style={local.container} accessibilityElementsHidden={controlsHidden}
    importantForAccessibility={controlsHidden ? 'no-hide-descendants' : 'auto'}>
    <View style={local.mapFrame} onLayout={({ nativeEvent: { layout } }) => { setSize(`${layout.width}:${layout.height}`); setMapSize({ width: layout.width, height: layout.height }); }}>
      <Map ref={mapRef} key={attempt} testID="route-map" style={local.map} mapStyle={routeMapStyle} androidView="texture"
        importantForAccessibility={controlsHidden ? 'no-hide-descendants' : 'auto'}
        attribution={false} logo={false} compass={false} dragPan touchZoom doubleTapZoom touchRotate={false} touchPitch={false}
        onDidFinishLoadingStyle={() => setReady(true)}
        onDidFinishRenderingMapFully={() => { setReady(true); setRendered(true); }}
        onDidFailLoadingMap={() => setError('Could not load the map. Check your connection and retry.')}
        onRegionIsChanging={() => { void updateLabels(); }}
        onRegionDidChange={() => { void updateLabels(); }}
        onRegionWillChange={({ nativeEvent }) => { if (nativeEvent.userInteraction) { automaticRef.current = false; setAutomatic(false); setSelectedId(null); } }}>
        <Camera ref={camera} initialViewState={{ center: [14.5058, 46.0569], zoom: 11 }} maxZoom={16} />
        <GeoJSONSource id="route-paths" data={geometry.paths}>
          <Layer id="route-lines" type="line" layout={{ 'line-cap': 'round', 'line-join': 'round' }}
            paint={{ 'line-color': ['get', 'color'], 'line-width': 4, 'line-opacity': 0.88 }} />
        </GeoJSONSource>
        <GeoJSONSource id="route-members" data={geometry.markers} onPress={({ nativeEvent }) => {
          const id = nativeEvent.features[0]?.properties?.memberId;
          if (typeof id === 'string') focusMember(id);
        }}>
          <Layer id="member-halos" type="circle" paint={{ 'circle-radius': 12, 'circle-color': '#071013', 'circle-opacity': 0.8 }} />
          <Layer id="member-dots" type="circle" paint={{ 'circle-radius': 8, 'circle-color': ['get', 'color'],
            'circle-stroke-width': 2, 'circle-stroke-color': '#f8fafc',
            'circle-opacity': ['case', ['==', ['get', 'status'], 'tracking'], 1, 0.6] }} />
        </GeoJSONSource>
        {!controlsHidden && locatedMembers.filter((member) => visibleLabels.includes(member.id)).map((member) => (
          <Marker key={member.id} id={`member-label-${member.id}`}
            lngLat={[member.latest!.longitude, member.latest!.latitude]} anchor="bottom" offset={[0, -memberLabelOffset]}
            onPress={() => focusMember(member.id)}>
            <MemberNamePill name={member.displayName} status={member.status} selected={member.id === selectedId} />
          </Marker>
        ))}
      </Map>
      <View pointerEvents="none" accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={local.labelMeasurements}>
        {locatedMembers.map((member) => <MemberNamePill key={member.id} name={member.displayName} status={member.status}
          onLayout={({ nativeEvent: { layout } }) => setLabelSizes((current) => {
            const previous = current[member.id];
            return previous?.width === layout.width && previous.height === layout.height ? current :
              { ...current, [member.id]: { width: layout.width, height: layout.height } };
          })} />)}
      </View>
      {!controlsHidden && !rendered && !error && <View pointerEvents="none" style={[local.notice, { top: topInset }]}>
        <ActivityIndicator color="#22c55e" /><Text style={styles.text}>Loading map…</Text>
      </View>}
      {!controlsHidden && rendered && !error && !geometry.bounds && <View pointerEvents="none" style={[local.notice, { top: topInset }]}>
        <Text style={styles.label}>No locations yet</Text>
        <Text style={styles.text}>{snapshot.route.status === 'closed' ? 'This archive has no saved locations.' : 'Locations will appear when a member shares from a connected client.'}</Text>
      </View>}
      <Animated.View style={[local.fit, { opacity, bottom: bottomInset + 56 }]} pointerEvents={showFit ? 'auto' : 'none'}
        accessibilityElementsHidden={!showFit} importantForAccessibility={showFit ? 'auto' : 'no-hide-descendants'}>
        <Pressable accessibilityRole="button" accessibilityLabel="Fit group" disabled={!showFit} style={local.fitButton}
          onPress={() => { setSelectedId(null); automaticRef.current = true; setAutomatic(true); fit(geometry.bounds, true); }}>
          <Text style={styles.label}>Fit group</Text>
        </Pressable>
      </Animated.View>
      {!controlsHidden && <Pressable accessibilityRole="link" accessibilityLabel="OpenStreetMap copyright and contributors"
        style={[local.attribution, { bottom: bottomInset + 4 }]}
        onPress={() => { void Linking.openURL('https://www.openstreetmap.org/copyright').catch(() => setError('Could not open the map attribution link.')); }}>
        <Text style={local.attributionText}>© OpenStreetMap contributors</Text>
      </Pressable>}
      {!controlsHidden && error && <View style={[local.notice, { top: topInset }]}>
        <ErrorMessage message={error} />
        <ActionButton label="Retry map" secondary onPress={() => {
          setReady(false); setRendered(false); setError(null); automaticRef.current = true;
          setAutomatic(true); setSelectedId(null); setAttempt((value) => value + 1);
        }} />
      </View>}
      {!controlsHidden && selected?.latest && !error && <View style={[local.notice, { top: topInset }]} accessibilityLiveRegion="polite">
        <Text style={styles.label}>{selected.displayName} · {presence[selected.status]}</Text>
        <Text style={styles.text}>Last location {new Date(selected.latest.recordedAt).toLocaleString()}</Text>
      </View>}
    </View>
  </View>;
}

function MemberNamePill({ name, status, selected = false, onLayout }: {
  name: string; status: string; selected?: boolean; onLayout?: (event: LayoutChangeEvent) => void;
}) {
  const writtenStatus = memberLocationStatus(status);
  return <View collapsable={false} onLayout={onLayout} style={[local.memberLabel, selected && local.selectedLabel]}>
    <Text accessibilityLabel={`${name}, ${writtenStatus || 'Sharing location'}`}
      numberOfLines={1} ellipsizeMode="tail" style={local.memberLabelName}>{name}</Text>
    {!!writtenStatus && <Text style={local.memberLabelStatus}>{writtenStatus}</Text>}
  </View>;
}

const local = StyleSheet.create({
  container: { position: 'absolute', top: 0, right: 0, bottom: 0, left: 0 },
  mapFrame: { flex: 1, backgroundColor: '#151b20' },
  map: { flex: 1 },
  labelMeasurements: { position: 'absolute', left: -1000, top: 0, alignItems: 'flex-start' },
  memberLabel: { maxWidth: 152, borderRadius: 9, borderWidth: 1, borderColor: '#34404a',
    backgroundColor: 'rgba(21, 27, 32, 0.75)', paddingHorizontal: 10, paddingVertical: 5, alignItems: 'center' },
  memberLabelName: { maxWidth: 130, color: '#f8fafc', fontSize: 13, fontWeight: '700', lineHeight: 18 },
  memberLabelStatus: { color: '#f8fafc', fontSize: 10, lineHeight: 14 },
  selectedLabel: { borderColor: '#f8fafc' },
  notice: { position: 'absolute', top: 20, left: 20, right: 20, borderRadius: 12, backgroundColor: '#151b20', padding: 16, gap: 8 },
  fit: { position: 'absolute', bottom: 20, right: 16 },
  fitButton: { minHeight: 48, backgroundColor: '#151b20', padding: 14, borderRadius: 12 },
  attribution: { position: 'absolute', left: 12, minHeight: 44, justifyContent: 'center', backgroundColor: '#151b20', borderRadius: 8, paddingHorizontal: 8 },
  attributionText: { color: '#b5c2cb', fontSize: 13, textDecorationLine: 'underline' },
});
