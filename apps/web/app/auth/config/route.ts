export const dynamic = "force-dynamic";

// Public runtime configuration keeps the production image independent of its deployment.
export function GET() {
  return Response.json({ googleClientId: process.env.AUTH_GOOGLE_WEB_CLIENT_ID ?? "" }, { headers: { "Cache-Control": "no-store" } });
}
