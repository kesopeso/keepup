import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "KeepUp",
    short_name: "KeepUp",
    description: "Live route sharing for groups on the move.",
    start_url: "/",
    display: "standalone",
    background_color: "#0c1014",
    theme_color: "#173023",
    icons: [
      {
        src: "/icons/keepup-192.png",
        sizes: "192x192",
        type: "image/png",
      },
      {
        src: "/icons/keepup-512.png",
        sizes: "512x512",
        type: "image/png",
      },
      {
        src: "/icons/keepup-maskable-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
