import { useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react';
import type { Ref } from 'react';
import { AccessibilityInfo, ActivityIndicator, Animated, Linking, Pressable, StyleSheet, Text, View } from 'react-native';
import { Camera, GeoJSONSource, Layer, LogManager, Map, TransformRequestManager } from '@maplibre/maplibre-react-native';
import type { CameraRef } from '@maplibre/maplibre-react-native';
import { ActionButton, ErrorMessage, styles } from './ui';
import type { RouteSnapshot } from '../domain/routes';
import { boundsForPoints, snapshotGeometry } from '../map/snapshot-geometry';
import type { Bounds } from '../map/snapshot-geometry';
import { routeMapStyle } from '../map/tile-provider';

export type RouteMapRef = { focusMember: (id: string) => void };
const presence = { tracking: 'Sharing location', spectating: 'Spectating', stale: 'Location delayed', offline: 'Offline', left: 'Left' };

export function RouteMap({ snapshot, ref }: { snapshot: RouteSnapshot; ref?: Ref<RouteMapRef> }) {
  const geometry = useMemo(() => snapshotGeometry(snapshot), [snapshot]);
  const camera = useRef<CameraRef>(null);
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
      padding: { top: 48, right: 36, bottom: 64, left: 36 } };
    const stop = west === east && south === north
      ? { ...options, center: [west, south] as [number, number], zoom: 15 }
      : { ...options, bounds };
    void camera.current.setStop(stop).catch(() => setError('Could not position the map. Please retry.'));
  }, [ready, reduceMotion]);

  // Refresh and size changes only move the camera while automatic fitting is enabled.
  useEffect(() => { if (automaticRef.current) fit(geometry.bounds, false); }, [geometry, fit, size]);
  const showFit = ready && !error && !automatic && !!geometry.bounds;
  useEffect(() => {
    const animation = Animated.timing(opacity, { toValue: showFit ? 1 : 0, duration: reduceMotion ? 0 : 180, useNativeDriver: true });
    animation.start();
    return () => animation.stop();
  }, [showFit, reduceMotion, opacity]);

  function focusMember(id: string) {
    const person = geometry.members.find((member) => member.id === id);
    if (!person?.latest || !ready || error) return;
    setSelectedId(id);
    setAutomatic(false);
    fit(boundsForPoints(person.points), true);
  }
  useImperativeHandle(ref, () => ({ focusMember }));

  return <View style={local.container}>
    <View style={local.mapFrame} onLayout={({ nativeEvent: { layout } }) => setSize(`${layout.width}:${layout.height}`)}>
      <Map key={attempt} testID="route-map" style={local.map} mapStyle={routeMapStyle} androidView="texture"
        attribution logo={false} compass={false} touchRotate={false} touchPitch={false}
        onDidFinishLoadingStyle={() => setReady(true)}
        onDidFinishRenderingMapFully={() => { setReady(true); setRendered(true); }}
        onDidFailLoadingMap={() => setError('Could not load the map. Check your connection and retry.')}
        onRegionWillChange={({ nativeEvent }) => { if (nativeEvent.userInteraction) { setAutomatic(false); setSelectedId(null); } }}>
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
      </Map>
      {!rendered && !error && <View pointerEvents="none" style={local.notice}>
        <ActivityIndicator color="#22c55e" /><Text style={styles.text}>Loading map…</Text>
      </View>}
      {rendered && !error && !geometry.bounds && <View pointerEvents="none" style={local.notice}>
        <Text style={styles.label}>No locations yet</Text>
        <Text style={styles.text}>{snapshot.route.status === 'closed' ? 'This archive has no saved locations.' : 'Locations will appear when a member shares from a connected client.'}</Text>
      </View>}
      <Animated.View style={[local.fit, { opacity }]} pointerEvents={showFit ? 'auto' : 'none'}
        accessibilityElementsHidden={!showFit} importantForAccessibility={showFit ? 'auto' : 'no-hide-descendants'}>
        <Pressable accessibilityRole="button" accessibilityLabel="Fit group" disabled={!showFit} style={local.fitButton}
          onPress={() => { setSelectedId(null); setAutomatic(true); fit(geometry.bounds, true); }}>
          <Text style={styles.label}>Fit group</Text>
        </Pressable>
      </Animated.View>
    </View>
    <Pressable accessibilityRole="link" accessibilityLabel="OpenStreetMap copyright and contributors" style={local.attribution}
      onPress={() => { void Linking.openURL('https://www.openstreetmap.org/copyright').catch(() => setError('Could not open the map attribution link.')); }}>
      <Text style={local.attributionText}>© OpenStreetMap contributors</Text>
    </Pressable>
    <ErrorMessage message={error} />
    {error && <ActionButton label="Retry map" secondary onPress={() => {
      setReady(false); setRendered(false); setError(null); setAutomatic(true); setSelectedId(null); setAttempt((value) => value + 1);
    }} />}
    {selected?.latest && <View style={styles.card} accessibilityLiveRegion="polite">
      <Text style={styles.label}>{selected.displayName} · {presence[selected.status]}</Text>
      <Text style={styles.text}>Last location {new Date(selected.latest.recordedAt).toLocaleString()}</Text>
    </View>}
  </View>;
}

const local = StyleSheet.create({
  container: { gap: 12 },
  mapFrame: { height: 360, borderRadius: 20, overflow: 'hidden', backgroundColor: '#151b20', borderWidth: 1, borderColor: '#34404a' },
  map: { flex: 1 },
  notice: { position: 'absolute', top: 20, left: 20, right: 20, borderRadius: 12, backgroundColor: '#151b20', padding: 16, gap: 8 },
  fit: { position: 'absolute', bottom: 20, right: 16 },
  fitButton: { minHeight: 48, backgroundColor: '#151b20', padding: 14, borderRadius: 12 },
  attribution: { minHeight: 48, justifyContent: 'center', alignItems: 'flex-start' },
  attributionText: { color: '#b5c2cb', fontSize: 13, textDecorationLine: 'underline' },
});
