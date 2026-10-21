import * as SecureStore from 'expo-secure-store';
import * as Crypto from 'expo-crypto';
import { requireNativeModule } from 'expo';
import { createAuthTransport, AccountError } from './api';
import { createAccountController } from './controller';
import { createAccountRepository } from './repository';

export async function nativeAccountController(baseUrl: string) {
  if (!__DEV__ && new URL(baseUrl).protocol !== 'https:') throw new Error('Account sessions require HTTPS in release builds.');
  const namespace = await Crypto.digestStringAsync(Crypto.CryptoDigestAlgorithm.SHA256, baseUrl);
  const options = { keychainAccessible: SecureStore.WHEN_UNLOCKED_THIS_DEVICE_ONLY };
  const repository = createAccountRepository({
    getItem: (key) => SecureStore.getItemAsync(key, options),
    setItem: (key, value) => SecureStore.setItemAsync(key, value, options),
    deleteItem: (key) => SecureStore.deleteItemAsync(key, options),
  }, namespace);
  return createAccountController(createAuthTransport(baseUrl), repository);
}

export async function pickGoogleProof(nonce: string): Promise<string | null> {
  const clientId = process.env.EXPO_PUBLIC_GOOGLE_SERVER_CLIENT_ID;
  if (!clientId || !/^[A-Za-z0-9_.-]+\.apps\.googleusercontent\.com$/.test(clientId)) throw new AccountError('google_unavailable');
  try {
    return await requireNativeModule<{ signIn(clientId: string, nonce: string): Promise<string | null> }>('KeepUpGoogle').signIn(clientId, nonce);
  } catch { throw new AccountError('google_unavailable'); }
}

export function observeAccountLinks(onLink: (url: string) => void) {
  const module = requireNativeModule('KeepUpGoogle');
  const subscription = module.addListener('authLink', ({ url }: { url: string }) => onLink(url));
  const initial: string | null = module.consumeInitialLink();
  if (initial) onLink(initial);
  return () => subscription.remove();
}
