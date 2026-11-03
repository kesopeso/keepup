export const dynamic = "force-dynamic";

// Public signing fingerprints associate this deployment with approved APKs.
export function GET() {
  const fingerprints = [...new Set((process.env.ANDROID_APP_LINK_SHA256_FINGERPRINTS ?? "")
    .split(",").map(value => value.trim().toUpperCase()).filter(Boolean))];
  const headers = { "Cache-Control": "no-store" };
  if (fingerprints.some(value => !/^(?:[0-9A-F]{2}:){31}[0-9A-F]{2}$/.test(value))) {
    return Response.json({ error: "invalid_app_link_configuration" }, { status: 503, headers });
  }
  return Response.json(fingerprints.length ? [{
    relation: ["delegate_permission/common.handle_all_urls"],
    target: {
      namespace: "android_app",
      package_name: "eu.kesopeso.keepup",
      sha256_cert_fingerprints: fingerprints,
    },
  }] : [], { headers });
}
