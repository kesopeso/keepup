import type { Metadata } from "next";
import { LandingPage } from "./landing/landing-page";

export const metadata: Metadata = {
  title: "KeepUp | Live route sharing for your group",
  description:
    "Keep your group in view on one live map. Create a route, share a link, and choose when to share your location. No account required.",
};

export default function Page() {
  return <LandingPage createHref="/routes/new" />;
}
