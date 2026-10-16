import { ImageResponse } from "next/og";
import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { createRequire } from "node:module";

export const alt = "KeepUp. Live route sharing for groups on the move.";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default async function OpenGraphImage() {
  const resolve = createRequire(join(process.cwd(), "package.json")).resolve;
  const [wordmarkFont, bodyFont] = await Promise.all([
    readFile(join(process.cwd(), "assets/fonts/Ubuntu-Bold.ttf")),
    // Preserve the renderer's existing default font for the description and URL.
    readFile(join(dirname(resolve("next/package.json")), "dist/compiled/@vercel/og/Geist-Regular.ttf")),
  ]);

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          padding: "76px 88px",
          background: "linear-gradient(125deg, #173023, #234c36)",
          color: "#f8fafc",
          fontFamily: "Geist",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 30 }}>
          <svg width="144" height="144" viewBox="0 0 128 128">
            <rect x="5" y="5" width="118" height="118" rx="34" fill="#173023" stroke="#3f7652" strokeWidth="6" />
            <path d="M42 31v66" fill="none" stroke="#f8fafc" strokeWidth="14" strokeLinecap="round" />
            <path d="M87 96C72 82 64 69 49 65C64 63 75 52 83 39" fill="none" stroke="#22c55e" strokeWidth="13" strokeLinecap="round" strokeLinejoin="round" />
            <path d="m78 35 14-4-2 15" fill="none" stroke="#22c55e" strokeWidth="9" strokeLinecap="round" strokeLinejoin="round" />
            <circle cx="70.4" cy="79.1" r="7" fill="#f8fafc" stroke="#173023" strokeWidth="4" />
          </svg>
          <span style={{ display: "flex", fontFamily: "Ubuntu", fontSize: 104, fontWeight: 700, letterSpacing: -5 }}>keep<span style={{ color: "#22c55e" }}>up</span></span>
        </div>
        <div style={{ display: "flex", marginTop: 42, fontSize: 48, lineHeight: 1.2, maxWidth: 880 }}>
          Live route sharing for groups on the move.
        </div>
        <div style={{ display: "flex", marginTop: 36, fontSize: 26, color: "#a7dcb8" }}>
          keepup.kesopeso.eu
        </div>
      </div>
    ),
    {
      ...size,
      fonts: [
        { name: "Geist", data: bodyFont, weight: 400, style: "normal" },
        { name: "Ubuntu", data: wordmarkFont, weight: 700, style: "normal" },
      ],
    },
  );
}
