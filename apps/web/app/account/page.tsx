import type { Metadata } from "next";
import AuthScreen from "../auth/auth-screen";
export const metadata: Metadata = { title: "Your account | KeepUp", robots: { index: false, follow: false }, referrer: "no-referrer" };
export default function AccountPage() { return <AuthScreen screen="account" />; }
