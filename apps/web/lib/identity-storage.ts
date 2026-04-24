export const transportModes = [
  "walking",
  "bicycle",
  "car",
  "bus",
  "train",
  "boat",
  "airplane",
] as const;

export type TransportMode = (typeof transportModes)[number];

export type BrowserProfile = {
  clientId: string;
  displayName: string;
  transportMode: TransportMode;
};

export type RouteAuth = {
  memberToken?: string;
  ownerToken?: string;
};

const clientIdKey = "keepup.clientId";
const profileKey = "keepup.profile";
const routeAuthPrefix = "keepup.routeAuth.";
const defaultTransportMode: TransportMode = "car";

function getStorage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

function readJson<T>(key: string): T | null {
  const storage = getStorage();
  if (!storage) {
    return null;
  }

  const rawValue = storage.getItem(key);
  if (!rawValue) {
    return null;
  }

  try {
    return JSON.parse(rawValue) as T;
  } catch {
    storage.removeItem(key);
    return null;
  }
}

function writeJson<T>(key: string, value: T): void {
  const storage = getStorage();
  if (!storage) {
    return;
  }

  try {
    storage.setItem(key, JSON.stringify(value));
  } catch {
    return;
  }
}

function isTransportMode(value: unknown): value is TransportMode {
  return (
    typeof value === "string" &&
    transportModes.includes(value as TransportMode)
  );
}

function normalizeRouteCode(code: string): string {
  return code.trim().toUpperCase();
}

function createClientId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }

  return `client_${Date.now().toString(36)}_${Math.random()
    .toString(36)
    .slice(2, 10)}`;
}

export function getOrCreateClientId(): string {
  const storage = getStorage();
  if (!storage) {
    return createClientId();
  }

  const existingClientId = storage.getItem(clientIdKey);
  if (existingClientId) {
    return existingClientId;
  }

  const clientId = createClientId();
  try {
    storage.setItem(clientIdKey, clientId);
  } catch {
    return clientId;
  }
  return clientId;
}

export function getProfile(): BrowserProfile {
  const clientId = getOrCreateClientId();
  const storedProfile = readJson<Partial<BrowserProfile>>(profileKey);

  return {
    clientId,
    displayName:
      typeof storedProfile?.displayName === "string"
        ? storedProfile.displayName
        : "",
    transportMode: isTransportMode(storedProfile?.transportMode)
      ? storedProfile.transportMode
      : defaultTransportMode,
  };
}

export function saveProfile(profile: {
  displayName: string;
  transportMode: TransportMode;
}): BrowserProfile {
  const nextProfile: BrowserProfile = {
    clientId: getOrCreateClientId(),
    displayName: profile.displayName.trim(),
    transportMode: profile.transportMode,
  };

  writeJson(profileKey, nextProfile);
  return nextProfile;
}

export function getRouteAuth(code: string): RouteAuth | null {
  const routeCode = normalizeRouteCode(code);
  if (!routeCode) {
    return null;
  }

  const storedAuth = readJson<RouteAuth>(`${routeAuthPrefix}${routeCode}`);
  if (!storedAuth?.memberToken && !storedAuth?.ownerToken) {
    return null;
  }

  return storedAuth;
}

export function saveRouteAuth(code: string, auth: RouteAuth): RouteAuth | null {
  const routeCode = normalizeRouteCode(code);
  if (!routeCode) {
    return null;
  }

  const existingAuth = getRouteAuth(routeCode) ?? {};
  const nextAuth: RouteAuth = { ...existingAuth };
  if (auth.memberToken) {
    nextAuth.memberToken = auth.memberToken;
  }
  if (auth.ownerToken) {
    nextAuth.ownerToken = auth.ownerToken;
  }

  if (!nextAuth.memberToken && !nextAuth.ownerToken) {
    return null;
  }

  writeJson(`${routeAuthPrefix}${routeCode}`, nextAuth);
  return nextAuth;
}

export function clearRouteAuth(code: string): void {
  const routeCode = normalizeRouteCode(code);
  const storage = getStorage();
  if (!routeCode || !storage) {
    return;
  }

  storage.removeItem(`${routeAuthPrefix}${routeCode}`);
}
