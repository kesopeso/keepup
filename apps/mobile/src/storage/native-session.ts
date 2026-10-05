import * as SecureStore from 'expo-secure-store';
import * as Crypto from 'expo-crypto';
import { createSessionRepository } from './session-repository';

export async function createNativeSessionRepository(baseUrl: string) {
  const namespace = await Crypto.digestStringAsync(Crypto.CryptoDigestAlgorithm.SHA256, baseUrl);
  return createSessionRepository({
    getItem: (key) => SecureStore.getItemAsync(key),
    setItem: (key, value) => SecureStore.setItemAsync(key, value),
    deleteItem: (key) => SecureStore.deleteItemAsync(key),
  }, namespace, () => Crypto.randomUUID());
}
