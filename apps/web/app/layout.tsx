import type { Metadata, Viewport } from "next";
import "maplibre-gl/dist/maplibre-gl.css";
import "./globals.css";

export const metadata: Metadata = {
  metadataBase: new URL("https://keepup.kesopeso.eu"),
  title: "KeepUp",
  description: "Live route sharing for groups on the move.",
  applicationName: "KeepUp",
  manifest: "/manifest.webmanifest",
  openGraph: {
    type: "website",
    siteName: "KeepUp",
    title: "KeepUp",
    description: "Live route sharing for groups on the move.",
  },
  twitter: {
    card: "summary_large_image",
    title: "KeepUp",
    description: "Live route sharing for groups on the move.",
    images: [{ url: "/opengraph-image", alt: "KeepUp. Live route sharing for groups on the move." }],
  },
  icons: {
    icon: [{ url: "/icon.svg", type: "image/svg+xml" }],
    apple: [{ url: "/apple-icon.png", sizes: "180x180" }],
  },
};

export const viewport: Viewport = {
  themeColor: "#173023",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
