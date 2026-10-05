const ANDROID_EMULATOR_API_URL = 'http://10.0.2.2:3000/api';

export function getApiBaseUrl(
  configuredUrl = process.env.EXPO_PUBLIC_API_URL,
  isDevelopment = __DEV__,
): string {
  const value = configuredUrl?.trim() || (isDevelopment ? ANDROID_EMULATOR_API_URL : '');
  if (!value) {
    throw new Error('EXPO_PUBLIC_API_URL is required for release builds.');
  }

  const url = new URL(value);
  if (
    !['http:', 'https:'].includes(url.protocol) ||
    !url.hostname || url.username || url.password || url.search || url.hash
  ) {
    throw new Error('EXPO_PUBLIC_API_URL must be an HTTP(S) API base URL.');
  }

  return url.toString().replace(/\/+$/, '');
}
