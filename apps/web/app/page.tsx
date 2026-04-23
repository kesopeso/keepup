const apiUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
const wsUrl = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080/ws";

export default function HomePage() {
  return (
    <main className="page-shell">
      <section className="hero">
        <p className="eyebrow">KeepUp</p>
        <h1>Live route sharing for groups on the move.</h1>
        <p className="lede">
          The product shell is running. Next up is wiring route creation, join,
          live tracking, and the map experience.
        </p>
      </section>

      <section className="status-grid" aria-label="Service status">
        <article className="status-card">
          <h2>Web</h2>
          <p>Next.js app scaffolded and ready for feature work.</p>
        </article>
        <article className="status-card">
          <h2>API</h2>
          <p>
            Backend expected at <code>{apiUrl}</code>
          </p>
        </article>
        <article className="status-card">
          <h2>Realtime</h2>
          <p>
            WebSocket endpoint reserved at <code>{wsUrl}</code>
          </p>
        </article>
      </section>
    </main>
  );
}
