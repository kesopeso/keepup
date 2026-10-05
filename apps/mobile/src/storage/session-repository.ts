import { isTransportMode } from '../domain/routes.ts';
import type { Membership, Profile } from '../domain/routes.ts';

export type KeyValueStore = {
  getItem: (key: string) => Promise<string | null>;
  setItem: (key: string, value: string) => Promise<void>;
  deleteItem: (key: string) => Promise<void>;
};

export class StorageError extends Error {
  constructor() {
    super('Could not read or save route access on this device. Please try again.');
    this.name = 'StorageError';
  }
}

export function createSessionRepository(store: KeyValueStore, namespace: string, createId: () => string) {
  const prefix = `keepup.${namespace}`;
  const remembered = new Map<string, Membership>();
  async function read(key: string): Promise<unknown> {
    try {
      const raw = await store.getItem(key);
      return raw === null ? null : JSON.parse(raw);
    } catch { throw new StorageError(); }
  }
  async function write(key: string, value: unknown) {
    try { await store.setItem(key, JSON.stringify(value)); }
    catch { throw new StorageError(); }
  }
  async function remove(key: string) {
    try { await store.deleteItem(key); }
    catch { throw new StorageError(); }
  }
  function memberKey(code: string) { return `${prefix}.route.${code}`; }
  function validMembership(value: unknown, code: string): value is Membership {
    if (!value || typeof value !== 'object') return false;
    const member = value as Partial<Membership>;
    return member.code === code && typeof member.memberId === 'string' && member.memberId.length > 0 &&
      typeof member.memberToken === 'string' && member.memberToken.length > 0;
  }
  return {
    async getProfile(): Promise<Profile> {
      const value = await read(`${prefix}.profile`) as Partial<Profile> | null;
      if (value === null) {
        const profile: Profile = { clientId: createId(), displayName: '', transportMode: 'car' };
        await write(`${prefix}.profile`, profile);
        return profile;
      }
      if (typeof value.clientId !== 'string' || !value.clientId || typeof value.displayName !== 'string' || !isTransportMode(value.transportMode)) {
        throw new StorageError();
      }
      return value as Profile;
    },
    async saveProfile(profile: Profile) { await write(`${prefix}.profile`, profile); },
    async getMembership(code: string): Promise<Membership | null> {
      if (remembered.has(code)) return remembered.get(code)!;
      const value = await read(memberKey(code));
      if (value === null) return null;
      if (!validMembership(value, code)) throw new StorageError();
      remembered.set(code, value);
      return value;
    },
    async saveMembership(member: Membership) {
      // Keep a successful join in memory even if the device write fails. Retry must not POST again.
      remembered.set(member.code, member);
      await write(memberKey(member.code), member);
      await write(`${prefix}.lastRoute`, member.code);
    },
    async getLastCode(): Promise<string | null> {
      const value = await read(`${prefix}.lastRoute`);
      if (value === null) return null;
      if (typeof value !== 'string' || !/^[A-Z0-9]{6}$/.test(value)) throw new StorageError();
      return value;
    },
    async forgetMembership(code: string) {
      await remove(memberKey(code));
      remembered.delete(code);
      if (await read(`${prefix}.lastRoute`) === code) await remove(`${prefix}.lastRoute`);
    },
  };
}

export type SessionRepository = ReturnType<typeof createSessionRepository>;
