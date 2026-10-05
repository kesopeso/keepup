export const HEALTH_CHECK_TIMEOUT_MS = 5_000;

export type UnavailableReason = 'timeout' | 'network' | 'server' | 'invalid_response' | 'configuration';
export type HealthCheckResult =
  | { status: 'connected' }
  | { status: 'unavailable'; reason: UnavailableReason };

export async function checkApiHealth(
  baseUrl: string,
  signal: AbortSignal,
): Promise<HealthCheckResult> {
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal.addEventListener('abort', abort);
  if (signal.aborted) abort();

  let timedOut = false;
  const timeout = setTimeout(() => {
    timedOut = true;
    controller.abort();
  }, HEALTH_CHECK_TIMEOUT_MS);

  try {
    const response = await fetch(`${baseUrl}/healthz`, {
      headers: { Accept: 'application/json' },
      cache: 'no-store',
      signal: controller.signal,
    });
    if (!response.ok) return { status: 'unavailable', reason: 'server' };

    const body: unknown = await response.json();
    if (typeof body !== 'object' || body === null || !('status' in body) || body.status !== 'ok') {
      return { status: 'unavailable', reason: 'invalid_response' };
    }
    return { status: 'connected' };
  } catch (error) {
    // Cancellation belongs to the screen lifecycle, not to the unavailable state.
    if (signal.aborted) throw error;
    return {
      status: 'unavailable',
      reason: timedOut ? 'timeout' : error instanceof SyntaxError ? 'invalid_response' : 'network',
    };
  } finally {
    clearTimeout(timeout);
    signal.removeEventListener('abort', abort);
  }
}
