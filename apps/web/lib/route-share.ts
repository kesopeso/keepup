import { copyWithDevelopmentFallback } from "./development-clipboard.ts";

export type ShareResult = { title: string; detail: string; tone: "success" | "warning" | "error"; manualUrl?: string } | null;
export async function shareRouteLink(code: string, title: string): Promise<ShareResult> {
  const url = new URL(`/routes/${encodeURIComponent(code)}`, window.location.origin).href;
  try {
    if (navigator.share) { await navigator.share({ title, url }); return null; }
    await navigator.clipboard.writeText(url);
    return { title: "Route link copied", detail: "Ready to paste and share", tone: "success" };
  } catch (error) {
    if (error instanceof Error && error.name === "AbortError") return null;
    if (copyWithDevelopmentFallback(url)) return { title: "Route link copied", detail: "Link copied using an unsafe fallback. Development only.", tone: "warning" };
    return { title: "Copy link manually", detail: "Select and copy the route link", tone: "error", manualUrl: url };
  }
}
