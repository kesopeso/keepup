import { notFound } from "next/navigation";
import AuthScreen, { type AuthScreenName } from "../auth-screen";
const screens: Record<string, AuthScreenName> = {
  "": "login", login: "login", signup: "signup", verify: "verify", forgot: "forgot", "forgot-password": "forgot", reset: "reset", "reset-password": "reset", "google/verify": "google-verify", google: "google",
};
export default async function AuthPage({ params }: { params: Promise<{ step?: string[] }> }) {
  const { step } = await params;
  const name = step?.join("/") ?? "";
  if (!Object.hasOwn(screens, name)) notFound();
  const screen = screens[name];
  return <AuthScreen key={screen} screen={screen} />;
}
