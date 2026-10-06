import * as Location from 'expo-location';
import { LocationAccessError } from './foreground-sharing';
import type { LocationSample } from './foreground-sharing';

export async function requestLocationPermission(): Promise<void> {
  const permission = await Location.requestForegroundPermissionsAsync();
  if (!permission.granted) throw new LocationAccessError(permission.canAskAgain
    ? 'Location permission was denied. Tap Start sharing location to try again.'
    : 'Location permission is blocked. Open app settings and allow location while using KeepUp.', !permission.canAskAgain);
}
export async function prepareLocation(signal: AbortSignal): Promise<LocationSample> {
  if (!await Location.hasServicesEnabledAsync()) throw new LocationAccessError('Device location is turned off. Enable Location in Android settings, then try again.', true);
  if (signal.aborted) throw new Error('Location request cancelled.');
  return new Promise((resolve, reject) => {
    let subscription: Location.LocationSubscription | null = null;
    let finished = false;
    const timer = setTimeout(() => finish(new LocationAccessError('Finding your location took too long. Try outdoors with precise location enabled.')), 15000);
    function finish(error?: Error, sample?: LocationSample) {
      if (finished) return;
      finished = true; clearTimeout(timer); signal.removeEventListener('abort', abort); subscription?.remove();
      if (error) reject(error); else resolve(sample!);
    }
    function abort() { finish(new Error('Location request cancelled.')); }
    signal.addEventListener('abort', abort);
    void Location.watchPositionAsync({ accuracy: Location.Accuracy.High, timeInterval: 1000,
      distanceInterval: 0, mayShowUserSettingsDialog: false },
      (sample) => finish(undefined, sample),
      () => finish(new LocationAccessError('Your location is unavailable. Enable device Location and try outdoors.')),
    ).then((watch) => { if (finished) watch.remove(); else subscription = watch; })
      .catch(() => finish(new LocationAccessError('Could not start GPS. Check device Location and permissions.')));
  });
}
export function watchLocation(sample: (value: LocationSample) => void, error: (message: string) => void) {
  return Location.watchPositionAsync({ accuracy: Location.Accuracy.High, timeInterval: 5000, distanceInterval: 0, mayShowUserSettingsDialog: false }, sample,
    () => error('Location updates stopped. Check device Location and permissions, then resume sharing.'));
}
