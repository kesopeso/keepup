import { destination, isSession } from './contracts.ts';
import type { AccountSession, Destination, GoogleFlow } from './contracts.ts';
export type SecureStorage = { getItem(key: string): Promise<string | null>; setItem(key: string, value: string): Promise<void>; deleteItem(key: string): Promise<void> };

export function createAccountRepository(storage: SecureStorage, namespace: string, now = Date.now) {
  const key = (part: string) => `keepup.account.v1.${namespace}.${part}`;
  async function read(part: string): Promise<unknown> {
    const raw = await storage.getItem(key(part));
    if (raw === null) return null;
    try { return JSON.parse(raw); } catch { await storage.deleteItem(key(part)); return null; }
  }
  return {
    async session(): Promise<AccountSession | null> {
      const value = await read('session');
      if (value === null) return null;
      if (isSession(value) && Date.parse(value.expiresAt) > now()) return value;
      await storage.deleteItem(key('session')); return null;
    },
    saveSession: (value: AccountSession) => storage.setItem(key('session'), JSON.stringify(value)),
    clearSession: () => storage.deleteItem(key('session')),
    async flow(): Promise<GoogleFlow | null> {
      const value = await read('google') as GoogleFlow | null;
      if (value && /^[A-Za-z0-9_-]{43}$/.test(value.flowToken) && Date.parse(value.expiresAt) > now()) return value;
      await storage.deleteItem(key('google')); return null;
    },
    saveFlow: (value: GoogleFlow) => storage.setItem(key('google'), JSON.stringify(value)),
    clearFlow: () => storage.deleteItem(key('google')),
    async destination(): Promise<Destination | null> {
      const value = await read('destination') as { path?: unknown; expiresAt?: number } | null;
      if (value && typeof value.expiresAt === 'number' && value.expiresAt > now()) return destination(value.path);
      await storage.deleteItem(key('destination')); return null;
    },
    saveDestination: (path: Destination) => storage.setItem(key('destination'), JSON.stringify({ path, expiresAt: now() + 86400000 })),
    clearDestination: () => storage.deleteItem(key('destination')),
  };
}
export type AccountRepository = ReturnType<typeof createAccountRepository>;
