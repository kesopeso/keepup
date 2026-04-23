import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "KeepUp",
  description: "Live route sharing for groups on the move.",
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
