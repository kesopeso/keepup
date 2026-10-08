import { BrandText as Text } from './components/Brand';
import { useEffect, useState } from 'react';
import { ActivityIndicator } from 'react-native';
import { ActionButton, ErrorMessage, Screen, styles } from './components/ui';
import { getApiBaseUrl } from './api/config';
import { createRoutesApi } from './api/routes';
import type { RoutesApi } from './api/routes';
import type { Membership, Profile } from './domain/routes';
import { createNativeSessionRepository } from './storage/native-session';
import type { SessionRepository } from './storage/session-repository';
import { JoinRouteScreen } from './screens/JoinRouteScreen';
import { RouteSnapshotScreen } from './screens/RouteSnapshotScreen';
import { ConnectionCheckScreen } from './screens/ConnectionCheckScreen';

type Environment = { api: RoutesApi; repository: SessionRepository; profile: Profile };

export function MobileApp() {
  const [environment, setEnvironment] = useState<Environment | null>(null);
  const [member, setMember] = useState<Membership | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const [initialCode, setInitialCode] = useState('');
  const [joinError, setJoinError] = useState<string | null>(null);
  const [checkingConnection, setCheckingConnection] = useState(false);

  useEffect(() => {
    let active = true;
    async function initialize() {
      const baseUrl = getApiBaseUrl();
      const repository = await createNativeSessionRepository(baseUrl);
      const profile = await repository.getProfile();
      const code = await repository.getLastCode();
      const saved = code ? await repository.getMembership(code) : null;
      if (active) {
        setMember(saved);
        setInitialCode(code ?? '');
        setEnvironment({ api: createRoutesApi(baseUrl), repository, profile });
      }
    }
    void initialize().catch(() => {
      if (active) setError('Could not restore your saved access or set up the app connection. Please try again.');
    });
    return () => { active = false; };
  }, [attempt]);

  if (!environment) return <Screen>
    <Text brand style={styles.title}>Opening KeepUp</Text>
    <ErrorMessage message={error} />
    {error ? <ActionButton label="Retry" onPress={() => { setError(null); setAttempt((previous) => previous + 1); }} /> :
      <ActivityIndicator color="#22c55e" accessibilityLabel="Restoring saved access" />}
  </Screen>;

  if (checkingConnection) return <ConnectionCheckScreen onBack={() => setCheckingConnection(false)} />;

  if (member) return <RouteSnapshotScreen key={member.code} api={environment.api} repository={environment.repository} member={member}
    onChooseRoute={(code = '', message) => { setInitialCode(code); setJoinError(message ?? null); setMember(null); }} />;

  return <JoinRouteScreen api={environment.api} repository={environment.repository} profile={environment.profile}
    initialCode={initialCode} initialError={joinError} onConnectionCheck={() => setCheckingConnection(true)}
    onOpen={(saved, profile) => { setEnvironment({ ...environment, profile }); setMember(saved); setJoinError(null); }} />;
}
