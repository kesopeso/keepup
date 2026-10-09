import type { NextConfig } from "next";
import path from "node:path";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  output: "standalone",
  outputFileTracingRoot: path.join(__dirname, "../.."),
  allowedDevOrigins: (process.env.KEEPUP_DEV_ORIGINS ?? "")
    .split(",")
    .map((host) => host.trim())
    .filter(Boolean),
};

export default nextConfig;
