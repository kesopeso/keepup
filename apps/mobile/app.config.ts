import type { ExpoConfig, ConfigContext } from 'expo/config';

export default ({ config }: ConfigContext): ExpoConfig => {
  const value = process.env.EXPO_PUBLIC_AUTH_LINK_ORIGIN;
  let host: string | undefined;
  if (value) {
    const origin = new URL(value);
    if (origin.protocol !== 'https:' || origin.pathname !== '/' || origin.search || origin.hash || origin.username || origin.password || origin.port) {
      throw new Error('EXPO_PUBLIC_AUTH_LINK_ORIGIN must be an HTTPS origin without a port.');
    }
    host = origin.hostname;
  }
  return {
    ...config, scheme: config.scheme ?? 'keepup', name: config.name ?? 'KeepUp', slug: config.slug ?? 'keepup',
    android: {
      ...config.android,
      intentFilters: host ? [{ action: 'VIEW', autoVerify: true, category: ['BROWSABLE', 'DEFAULT'],
        data: [{ scheme: 'https', host, pathPrefix: '/auth/' }, { scheme: 'https', host, pathPrefix: '/routes/' }] }] : [],
    },
  };
};
