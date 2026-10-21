import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "My routes | KeepUp",
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};

export default function MyRoutesLayout({ children }: { children: React.ReactNode }) {
  return children;
}
