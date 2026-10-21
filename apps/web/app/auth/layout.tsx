import type { Metadata } from "next";
export const metadata: Metadata = { title: "Your account | KeepUp", robots: { index: false, follow: false }, referrer: "no-referrer" };
export default function AuthLayout({ children }: { children: React.ReactNode }) { return children; }
