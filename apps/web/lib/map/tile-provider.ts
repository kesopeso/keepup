import type { StyleSpecification } from "maplibre-gl";

export const routeMapStyle: StyleSpecification = {
  version: 8,
  sources: {
    "openstreetmap-raster": {
      type: "raster",
      tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
      tileSize: 256,
      attribution: "OpenStreetMap contributors",
    },
  },
  layers: [
    {
      id: "openstreetmap-raster",
      type: "raster",
      source: "openstreetmap-raster",
      paint: {
        "raster-brightness-min": 0.08,
        "raster-brightness-max": 0.78,
        "raster-contrast": 0.18,
        "raster-saturation": -0.18,
      },
    },
  ],
};
