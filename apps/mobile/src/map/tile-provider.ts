import type { ComponentProps } from 'react';
import type { Map } from '@maplibre/maplibre-react-native';

// Keep provider configuration separate from route geometry and UI.
export const routeMapStyle: ComponentProps<typeof Map>['mapStyle'] = {
  version: 8,
  sources: {
    'openstreetmap-raster': {
      type: 'raster', tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'], tileSize: 256,
      attribution: '<a href="https://www.openstreetmap.org/copyright">© OpenStreetMap contributors</a>',
    },
  },
  layers: [{ id: 'openstreetmap-raster', type: 'raster', source: 'openstreetmap-raster',
    paint: { 'raster-brightness-min': 0.08, 'raster-brightness-max': 0.78, 'raster-contrast': 0.18, 'raster-saturation': -0.18 } }],
};
