export type NavigationPosition = {
  latitude: number;
  longitude: number;
  accuracyM?: number;
  altitudeM?: number;
  speedMps?: number;
  headingDeg?: number;
  clientRecordedAt: string;
};

export type NavigationService = {
  watchPosition: (
    onPosition: (position: NavigationPosition) => void,
    onError: () => void,
  ) => () => void;
};

const fakePositionIntervalMs = 2_000;
const fakeStepMeters = 10;
const fakeBaseHeadingDeg = 45;
const fakeTurnJitterDeg = 35;
const fakeHeadingCorrection = 0.35;

type FakeRouteState = {
  position: NavigationPosition;
  headingDeg: number;
};

export const navigationService: NavigationService =
  process.env.NODE_ENV === "development"
    ? createDevelopmentNavigationService()
    : createBrowserNavigationService();

function createBrowserNavigationService(): NavigationService {
  return {
    watchPosition(onPosition, onError) {
      if (!("geolocation" in navigator)) {
        onError();
        return () => {};
      }

      const watchId = navigator.geolocation.watchPosition(
        (position) => {
          onPosition(navigationPositionFromGeolocation(position));
        },
        onError,
        {
          enableHighAccuracy: true,
          maximumAge: 5_000,
          timeout: 20_000,
        },
      );

      return () => {
        navigator.geolocation.clearWatch(watchId);
      };
    },
  };
}

function createDevelopmentNavigationService(): NavigationService {
  const browserNavigationService = createBrowserNavigationService();

  return {
    watchPosition(onPosition, onError) {
      let fakeRouteState: FakeRouteState | null = null;
      let fakePositionTimer: ReturnType<typeof setInterval> | null = null;
      let stopBrowserWatch = () => {};

      stopBrowserWatch = browserNavigationService.watchPosition((position) => {
        if (fakeRouteState) {
          return;
        }

        fakeRouteState = {
          position,
          headingDeg: position.headingDeg ?? fakeBaseHeadingDeg,
        };
        onPosition(position);
        stopBrowserWatch();

        fakePositionTimer = setInterval(() => {
          if (!fakeRouteState) {
            return;
          }

          fakeRouteState = nextFakeRouteState(fakeRouteState);
          onPosition(fakeRouteState.position);
        }, fakePositionIntervalMs);
      }, onError);

      return () => {
        stopBrowserWatch();
        if (fakePositionTimer) {
          clearInterval(fakePositionTimer);
        }
      };
    },
  };
}

function navigationPositionFromGeolocation(
  position: GeolocationPosition,
): NavigationPosition {
  const { coords } = position;

  return {
    latitude: coords.latitude,
    longitude: coords.longitude,
    accuracyM: finiteOrUndefined(coords.accuracy),
    altitudeM: finiteOrUndefined(coords.altitude),
    speedMps: finiteOrUndefined(coords.speed),
    headingDeg: finiteOrUndefined(coords.heading),
    clientRecordedAt: new Date(position.timestamp).toISOString(),
  };
}

function nextFakeRouteState(state: FakeRouteState): FakeRouteState {
  const headingCorrection =
    normalizeHeadingDelta(fakeBaseHeadingDeg - state.headingDeg) *
    fakeHeadingCorrection;
  const headingJitter = randomBetween(-fakeTurnJitterDeg, fakeTurnJitterDeg);
  const nextHeading = normalizeHeading(
    state.headingDeg + headingCorrection + headingJitter,
  );
  const stepMeters = randomBetween(fakeStepMeters * 0.7, fakeStepMeters * 1.3);
  const position = positionAtBearing(
    state.position,
    nextHeading,
    stepMeters,
  );

  return {
    position: {
      ...position,
      headingDeg: nextHeading,
      speedMps: stepMeters / (fakePositionIntervalMs / 1_000),
      clientRecordedAt: new Date().toISOString(),
    },
    headingDeg: nextHeading,
  };
}

function positionAtBearing(
  position: NavigationPosition,
  headingDeg: number,
  distanceMeters: number,
): NavigationPosition {
  const headingRadians = (headingDeg * Math.PI) / 180;
  const northMeters = Math.cos(headingRadians) * distanceMeters;
  const eastMeters = Math.sin(headingRadians) * distanceMeters;

  return {
    ...position,
    latitude: position.latitude + metersToLatitudeDegrees(northMeters),
    longitude:
      position.longitude + metersToLongitudeDegrees(eastMeters, position.latitude),
  };
}

function normalizeHeading(headingDeg: number) {
  return (headingDeg + 360) % 360;
}

function normalizeHeadingDelta(deltaDeg: number) {
  return ((deltaDeg + 540) % 360) - 180;
}

function randomBetween(min: number, max: number) {
  return min + Math.random() * (max - min);
}

function metersToLatitudeDegrees(meters: number) {
  return meters / 111_320;
}

function metersToLongitudeDegrees(meters: number, latitude: number) {
  const latitudeRadians = (latitude * Math.PI) / 180;
  return meters / (111_320 * Math.cos(latitudeRadians));
}

function finiteOrUndefined(value: number | null): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}
