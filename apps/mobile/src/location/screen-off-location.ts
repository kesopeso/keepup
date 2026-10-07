import { AppState, Platform } from 'react-native';
import * as Location from 'expo-location';
import * as TaskManager from 'expo-task-manager';
import { createLocationTask } from './location-task';
import { LocationAccessError } from './foreground-sharing';

const taskName = 'keepup-location-sharing';
const task = createLocationTask({
  async start() {
    if (AppState.currentState !== 'active') throw new LocationAccessError('Keep KeepUp open until sharing starts, then lock the screen.');
    await Location.startLocationUpdatesAsync(taskName, {
      accuracy: Location.Accuracy.High, timeInterval: 5000, distanceInterval: 0,
      deferredUpdatesInterval: 0, deferredUpdatesDistance: 0, deferredUpdatesTimeout: 0,
      mayShowUserSettingsDialog: false,
      foregroundService: {
        notificationTitle: 'KeepUp location sharing',
        notificationBody: 'Sharing is on, including with the screen locked. Tap to open KeepUp and stop.',
        notificationColor: '#22c55e', killServiceOnDestroy: true,
      },
    });
  },
  async stop() {
    if (await Location.hasStartedLocationUpdatesAsync(taskName)) await Location.stopLocationUpdatesAsync(taskName);
  },
});

// Register outside React so Android can dispatch tasks when the activity is not visible.
if (Platform.OS === 'android') {
  TaskManager.defineTask<{ locations: Location.LocationObject[] }>(taskName, async ({ data, error }) => {
    await task.deliver(data?.locations, error ? 'Location updates stopped. Open KeepUp, check Location permissions, and resume sharing.' : undefined);
  });
  void task.reset().catch(() => task.fail('Could not reset location sharing. Restart KeepUp before sharing.'));
}

export const watchScreenOffLocation = task.watch;
export async function checkScreenOffLocation() {
  if (!task.isActive()) return;
  try {
    const permission = await Location.getForegroundPermissionsAsync();
    const services = await Location.hasServicesEnabledAsync();
    const registered = await Location.hasStartedLocationUpdatesAsync(taskName);
    if (!permission.granted || !services || !registered) task.fail('Location sharing was interrupted. Check device Location and permissions, then resume sharing.');
  } catch { task.fail('Could not check location sharing. Stop and resume sharing.'); }
}
