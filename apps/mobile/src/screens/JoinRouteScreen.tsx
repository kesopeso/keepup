import { TransportSelect } from '../components/TransportSelect';
import { BrandText as Text } from '../components/Brand';
import { useEffect, useRef, useState } from 'react';
import { Keyboard, View } from 'react-native';
import { ActionButton, ErrorMessage, Field, Screen, styles } from '../components/ui';
import type { RoutesApi } from '../api/routes';
import { joinOrResume } from '../domain/join-session';
import { parseRouteCode } from '../domain/routes';
import type { Membership, Profile, RouteAccess } from '../domain/routes';
import type { SessionRepository } from '../storage/session-repository';

export function JoinRouteScreen({ api, repository, profile, initialCode, initialError, onOpen, onConnectionCheck, onCreate }: {
  api: RoutesApi; repository: SessionRepository; profile: Profile; initialCode: string; initialError: string | null;
  onOpen: (member: Membership, profile: Profile) => void; onConnectionCheck: () => void; onCreate: () => void;
}) {
  const [routeInput, setRouteInput] = useState(initialCode);
  const [access, setAccess] = useState<RouteAccess | null>(null);
  const [displayName, setDisplayName] = useState(profile.displayName);
  const [transportMode, setTransportMode] = useState(profile.transportMode);
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(initialError);
  const [retryAvailable, setRetryAvailable] = useState(false);
  const inFlight = useRef(false);
  const controller = useRef<AbortController | null>(null);
  const mounted = useRef(true);

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; controller.current?.abort(); };
  }, []);

  async function submit() {
    if (inFlight.current) return;
    let code: string;
    try {
      code = parseRouteCode(routeInput);
      if (access && !displayName.trim()) throw new Error('Enter your display name.');
      if (access?.requiresPassword && !password) throw new Error('Enter the route password.');
    } catch (error) {
      setError((error as Error).message);
      setRetryAvailable(false);
      return;
    }
    Keyboard.dismiss();
    inFlight.current = true;
    controller.current = new AbortController();
    const signal = controller.current.signal;
    setBusy(true);
    setError(null);
    setRetryAvailable(false);
    try {
      if (!access) {
        const saved = await repository.getMembership(code);
        if (saved) {
          await repository.saveMembership(saved);
          if (mounted.current) onOpen(saved, profile);
          return;
        }
        const details = await api.getAccess(code, signal);
        if (mounted.current) {
          setRouteInput(code);
          setAccess(details);
        }
      } else {
        const nextProfile = { ...profile, displayName: displayName.trim(), transportMode };
        const member = await joinOrResume(api, repository, code, { ...nextProfile, password }, signal);
        if (mounted.current) {
          setPassword('');
          onOpen(member, nextProfile);
        }
      }
    } catch (error) {
      if (mounted.current && !signal.aborted) {
        setError(error instanceof Error ? error.message : 'Could not join the route. Please try again.');
        setRetryAvailable(true);
      }
    } finally {
      inFlight.current = false;
      if (mounted.current) setBusy(false);
    }
  }

  function changeRoute() {
    setAccess(null);
    setPassword('');
    setError(null);
    setRetryAvailable(false);
  }

  return <Screen>
    <Text style={styles.title} accessibilityRole="header">{access ? access.name : 'Join a route'}</Text>
    <Text style={styles.text}>Your location stays private until you choose to share it.</Text>
    <View style={styles.card}>
      {!access ? <Field label="Route code or link" value={routeInput} onChangeText={setRouteInput}
        placeholder="ABC123 or a route link" autoCapitalize="none" editable={!busy} returnKeyType="go" onSubmitEditing={() => void submit()} /> : <>
        <Text style={styles.text}>Code {access.code} · {access.status === 'active' ? 'Active' : 'Closed'}</Text>
        {!!access.description && <Text style={styles.text}>{access.description}</Text>}
        {access.sharingPolicy === 'joiners_can_view_only' && <Text style={styles.text}>Only the owner can share a location on this route.</Text>}
        {access.status === 'active' ? <>
          <Field label="Display name" value={displayName} onChangeText={setDisplayName} placeholder="Your name"
            autoCapitalize="words" editable={!busy} returnKeyType="next" />
          <TransportSelect value={transportMode} onChange={setTransportMode} disabled={busy} />
          {access.requiresPassword && <Field label="Route password" value={password} onChangeText={setPassword}
            secureTextEntry autoCapitalize="none" autoComplete="current-password" editable={!busy}
            returnKeyType="go" onSubmitEditing={() => void submit()} />}
        </> : <Text style={styles.text}>This route is closed and is not accepting new members. Saved members can still view its archive.</Text>}
      </>}
      <ErrorMessage message={error} />
      {access?.status !== 'closed' && <ActionButton label={busy ? (access ? 'Joining…' : 'Loading…') : retryAvailable ? 'Retry' : access ? 'Join route' : 'Continue'}
        disabled={busy} onPress={() => void submit()} />}
      {access && <ActionButton label="Change route" disabled={busy} secondary onPress={changeRoute} />}
    </View>
    <ActionButton label="Create a new route" secondary disabled={busy} onPress={onCreate} />
    <ActionButton label="Check server connection" secondary disabled={busy} onPress={onConnectionCheck} />
  </Screen>;
}
