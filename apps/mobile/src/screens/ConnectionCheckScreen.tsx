import { useEffect, useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import {
  ActivityIndicator,
  Platform,
  Pressable,
  ScrollView,
  StatusBar as NativeStatusBar,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { getApiBaseUrl } from '../api/config';
import { checkApiHealth, type HealthCheckResult, type UnavailableReason } from '../api/health';

type ConnectionState = { status: 'checking' } | HealthCheckResult;

const unavailableMessages: Record<UnavailableReason, string> = {
  timeout: 'KeepUp took too long to respond. Please try again.',
  network: 'Could not reach KeepUp. Check your connection and try again.',
  server: 'KeepUp is temporarily unavailable. Please try again shortly.',
  invalid_response: 'KeepUp returned an unexpected response. Please try again.',
  configuration: 'The app connection is not configured. Contact the app developer.',
};

export function ConnectionCheckScreen() {
  const [state, setState] = useState<ConnectionState>({ status: 'checking' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();

    async function checkConnection() {
      let baseUrl: string;
      try {
        baseUrl = getApiBaseUrl();
      } catch {
        setState({ status: 'unavailable', reason: 'configuration' });
        return;
      }

      try {
        const result = await checkApiHealth(baseUrl, controller.signal);
        if (!controller.signal.aborted) setState(result);
      } catch {
        // An unmounted screen or a replaced check must not update this screen.
      }
    }

    void checkConnection();
    return () => controller.abort();
  }, [attempt]);

  const checking = state.status === 'checking';
  const connected = state.status === 'connected';
  const title = checking ? 'Connecting to KeepUp' : connected ? 'Connected to KeepUp' : 'KeepUp is unavailable';
  const message = state.status === 'unavailable'
    ? unavailableMessages[state.reason]
    : checking ? 'Checking the server connection…' : 'The server is ready.';
  const buttonLabel = checking ? 'Checking…' : connected ? 'Check again' : 'Retry';

  function retry() {
    if (checking) return;
    setState({ status: 'checking' });
    setAttempt((previous) => previous + 1);
  }

  return (
    <View style={styles.container}>
      <StatusBar style="light" />
      <ScrollView contentContainerStyle={styles.content}>
        <Text style={styles.brand} accessibilityRole="header">KeepUp</Text>
        <Text style={styles.subtitle}>Live route sharing</Text>

        <View style={styles.card}>
          <View style={styles.statusRow}>
            {checking ? <ActivityIndicator color="#22c55e" accessibilityLabel="Checking connection" /> : (
              <View style={[styles.dot, connected ? styles.connectedDot : styles.unavailableDot]} />
            )}
            <Text style={styles.statusLabel}>Server connection</Text>
          </View>
          <View accessibilityLiveRegion="polite" accessibilityRole="text">
            <Text style={styles.title}>{title}</Text>
            <Text style={styles.message}>{message}</Text>
          </View>

          <Pressable
            accessibilityRole="button"
            accessibilityLabel={buttonLabel}
            accessibilityState={{ disabled: checking, busy: checking }}
            disabled={checking}
            onPress={retry}
            style={({ pressed }) => [styles.button, checking && styles.disabledButton, pressed && styles.pressedButton]}
          >
            <Text style={styles.buttonText}>{buttonLabel}</Text>
          </Pressable>
        </View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#0c1014' },
  content: {
    flexGrow: 1,
    justifyContent: 'center',
    width: '100%',
    maxWidth: 520,
    alignSelf: 'center',
    paddingHorizontal: 24,
    paddingTop: (Platform.OS === 'android' ? NativeStatusBar.currentHeight ?? 0 : 44) + 24,
    paddingBottom: 48,
  },
  brand: { color: '#f8fafc', fontSize: 36, fontWeight: '800', letterSpacing: -1 },
  subtitle: { color: '#b5c2cb', fontSize: 16, marginTop: 8, marginBottom: 32 },
  card: { backgroundColor: '#151b20', borderColor: '#34404a', borderWidth: 1, borderRadius: 20, padding: 24 },
  statusRow: { flexDirection: 'row', alignItems: 'center', gap: 10, marginBottom: 24 },
  statusLabel: { color: '#b5c2cb', fontSize: 14 },
  dot: { width: 10, height: 10, borderRadius: 5 },
  connectedDot: { backgroundColor: '#22c55e' },
  unavailableDot: { backgroundColor: '#f6bf65' },
  title: { color: '#f8fafc', fontSize: 24, fontWeight: '700' },
  message: { color: '#b5c2cb', fontSize: 16, lineHeight: 24, marginTop: 12 },
  button: { minHeight: 48, backgroundColor: '#22c55e', borderRadius: 12, padding: 14, alignItems: 'center', justifyContent: 'center', marginTop: 28 },
  disabledButton: { opacity: 0.55 },
  pressedButton: { opacity: 0.8 },
  buttonText: { color: '#072713', fontSize: 16, fontWeight: '700' },
});
