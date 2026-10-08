import { useEffect, useRef, useState } from 'react';
import { BackHandler, Keyboard, Pressable, View } from 'react-native';
import { BrandText as Text } from '../components/Brand';
import { TransportSelect } from '../components/TransportSelect';
import { ActionButton, ErrorMessage, Field, Screen, styles } from '../components/ui';
import type { RoutesApi } from '../api/routes';
import { createRouteSession } from '../domain/create-session';
import type { Membership, Profile, SharingPolicy } from '../domain/routes';
import type { SessionRepository } from '../storage/session-repository';

const policies: { value: SharingPolicy; label: string; description: string }[] = [
  { value: 'everyone_can_share', label: 'Everyone can share', description: 'Members choose when to share their location.' },
  { value: 'joiners_can_view_only', label: 'Only the owner can share', description: 'Other members can view the route.' },
];

export function CreateRouteScreen({ api, repository, profile, onOpen, onBack }: {
  api: RoutesApi; repository: SessionRepository; profile: Profile;
  onOpen: (member: Membership, profile: Profile) => void; onBack: () => void;
}) {
  const [name, setName] = useState('');
  const [displayName, setDisplayName] = useState(profile.displayName);
  const [transportMode, setTransportMode] = useState(profile.transportMode);
  const [description, setDescription] = useState('');
  const [password, setPassword] = useState('');
  const [sharingPolicy, setSharingPolicy] = useState<SharingPolicy>('everyone_can_share');
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [createdCode, setCreatedCode] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const session = useRef(createRouteSession(api, repository));
  const controller = useRef<AbortController | null>(null);
  const inFlight = useRef(false);
  const mounted = useRef(true);
  const locked = busy || createdCode !== null;

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; controller.current?.abort(); };
  }, []);
  useEffect(() => {
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
      if (!inFlight.current && !session.current.getCreatedCode()) onBack();
      return true;
    });
    return () => subscription.remove();
  }, [onBack]);

  async function submit() {
    if (inFlight.current) return;
    Keyboard.dismiss();
    inFlight.current = true;
    controller.current = new AbortController();
    const signal = controller.current.signal;
    setBusy(true);
    setError(null);
    try {
      const result = await session.current.submit({ ...profile, name, displayName, transportMode, description, password, sharingPolicy }, signal);
      if (mounted.current) {
        setPassword('');
        onOpen(result.member, result.profile);
      }
    } catch (error) {
      if (mounted.current && !signal.aborted) {
        const code = session.current.getCreatedCode();
        setCreatedCode(code);
        if (code) setPassword('');
        setError(error instanceof Error ? error.message : 'Could not create the route. Please try again.');
      }
    } finally {
      inFlight.current = false;
      if (mounted.current) setBusy(false);
    }
  }

  return <Screen>
    <Text style={styles.title} accessibilityRole="header">Create a route</Text>
    <Text style={styles.text}>Create a route, share its code, and see your group on the map.</Text>
    <View style={styles.card}>
      <Field label="Route name" placeholder="Morning convoy" value={name} onChangeText={setName} editable={!locked} autoCapitalize="sentences" />
      <Field label="Display name" placeholder="Your name" value={displayName} onChangeText={setDisplayName} editable={!locked} autoCapitalize="words" />
      <TransportSelect value={transportMode} onChange={setTransportMode} disabled={locked} />
      <Pressable accessibilityRole="button" accessibilityLabel="Route settings" accessibilityState={{ expanded: settingsOpen, disabled: locked }}
        disabled={locked} onPress={() => setSettingsOpen(!settingsOpen)} style={[styles.button, styles.secondaryButton]}>
        <Text style={styles.label}>Route settings {settingsOpen ? '▴' : '▾'}</Text>
        <Text style={styles.text}>{sharingPolicy === 'everyone_can_share' ? 'Everyone can share' : 'Only the owner can share'}{password ? ' · Password protected' : ''}</Text>
      </Pressable>
      {settingsOpen && <>
        <Field label="Description (optional)" placeholder="Where is your group headed?" value={description} onChangeText={setDescription}
          editable={!locked} multiline numberOfLines={3} style={{ minHeight: 96, textAlignVertical: 'top' }} />
        <Field label="Password (optional)" placeholder="Protect access to this route" value={password} onChangeText={setPassword}
          editable={!locked} secureTextEntry autoCapitalize="none" autoComplete="new-password" />
        <Text style={styles.label}>Who can share location?</Text>
        {policies.map((policy) => <Pressable key={policy.value} accessibilityRole="radio" accessibilityLabel={policy.label}
          accessibilityState={{ checked: policy.value === sharingPolicy, disabled: locked }} disabled={locked}
          onPress={() => setSharingPolicy(policy.value)}
          style={[styles.input, policy.value === sharingPolicy && { borderColor: '#22c55e', backgroundColor: '#173023' }]}>
          <Text style={styles.label}>{policy.label}</Text>
          <Text style={styles.text}>{policy.description}</Text>
        </Pressable>)}
      </>}
      {createdCode && <Text style={styles.text}>Route {createdCode} was created. Retry saving its access to open it.</Text>}
      <ErrorMessage message={error} />
      <ActionButton label={busy ? (createdCode ? 'Saving access…' : 'Creating…') : createdCode ? 'Retry saving access' : 'Create route'}
        disabled={busy} onPress={() => void submit()} />
      <Text style={styles.text}>Your location stays private until you start sharing.</Text>
    </View>
    <ActionButton label="Join an existing route" secondary disabled={locked} onPress={onBack} />
  </Screen>;
}
