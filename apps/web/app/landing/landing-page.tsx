import type { ReactNode } from "react";
import { JoinRouteForm } from "./join-route-form";
import { LandingMapPreview } from "./landing-map-preview";
import styles from "./landing.module.css";

function PageBand({
  className,
  children,
}: {
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={className}>
      <div className={styles.content}>{children}</div>
    </div>
  );
}

function Brand() {
  return (
    <a className={styles.brand} href="#top" aria-label="KeepUp">
      <img src="/brand/keepup-mark.svg" alt="" width="36" height="36" />
      <span><b>Keep</b>Up</span>
    </a>
  );
}

function CreateLink({ href }: { href: string }) {
  return (
    <a className={styles.primary} href={href}>
      Create a route <span aria-hidden="true">↗</span>
    </a>
  );
}

// Pass the create destination explicitly so this page can move to / later.
export function LandingPage({ createHref }: { createHref: string }) {
  return (
    <main id="top" className={styles.page}>
      <PageBand>
        <nav className={styles.siteNav} aria-label="Main navigation">
          <Brand />
          <div className={styles.navLinks}>
            <a href="#how-it-works">How it works</a>
            <a href="#your-privacy">Your privacy</a>
          </div>
          <a className={styles.navCreate} href={createHref}>
            Create a route <span aria-hidden="true">↗</span>
          </a>
        </nav>
      </PageBand>

      <PageBand className={styles.heroBand}>
        <section className={styles.hero} aria-labelledby="landing-title">
          <div className={styles.heroCopy}>
            <span className={styles.eyebrow}>Keep your group in view</span>
            <h1 id="landing-title">Your people.<br /><em>Right there.</em></h1>
            <p className={styles.heroDescription}>
              A live map for your next shared trip. Start a route in your browser
              and invite your group with a link.
            </p>
            <div className={styles.actionCard}>
              <strong>Where are we going?</strong>
              <CreateLink href={createHref} />
              <div className={styles.orLine}><span>Already invited?</span></div>
              <JoinRouteForm />
              <p className={styles.microcopy}>
                Your location stays private until you start sharing.
              </p>
            </div>
          </div>
          <LandingMapPreview />
        </section>
      </PageBand>

      <PageBand className={styles.useStripBand}>
        <div className={styles.useStrip}>
          <span>One route. Your whole group.</span>
          <span>Share a link</span>
          <span>No account needed</span>
          <span>Choose when to share</span>
        </div>
      </PageBand>

      <PageBand>
        <section className={styles.howSection} id="how-it-works" aria-labelledby="how-title">
          <div className={styles.sectionHead}>
            <span className={styles.eyebrow}>A link is all it takes</span>
            <h2 id="how-title">Start together.<br />Stay in the loop.</h2>
          </div>
          <div className={styles.steps}>
            <article>
              <span className={styles.stepNo}>01</span>
              <h3>Create your route</h3>
              <p>Give it a name and choose who can share location. Add a password if you want one.</p>
            </article>
            <article>
              <span className={styles.stepNo}>02</span>
              <h3>Send the link</h3>
              <p>Friends join with their name and transport mode. No account needed.</p>
            </article>
            <article>
              <span className={styles.stepNo}>03</span>
              <h3>See the group move</h3>
              <p>Share your location when you&apos;re ready, or watch as a spectator.</p>
            </article>
          </div>
        </section>
      </PageBand>

      <PageBand className={styles.privacyBand}>
        <section className={styles.privacySection} id="your-privacy" aria-labelledby="privacy-title">
          <div className={styles.privacySymbol} aria-hidden="true">
            <svg viewBox="0 0 48 48">
              <path d="M24 5L39 11V23C39 34 24 42 24 42S9 34 9 23V11Z" fill="none" stroke="currentColor" strokeWidth="2" />
              <path d="M17 24L22 29 32 18" fill="none" stroke="currentColor" strokeWidth="2" />
            </svg>
          </div>
          <div>
            <span className={styles.eyebrow}>You&apos;re in control</span>
            <h2 id="privacy-title">Your location.<br />Your decision.</h2>
            <p>
              Joining a route doesn&apos;t share your location. Start sharing
              explicitly, stop whenever you want, or just follow along.
            </p>
          </div>
          <div className={styles.privacyFacts}>
            <p><span aria-hidden="true">✓</span> Access by link or code</p>
            <p><span aria-hidden="true">✓</span> Optional route password</p>
            <p><span aria-hidden="true">✓</span> No account required</p>
          </div>
        </section>
      </PageBand>

      <PageBand className={styles.finalCtaBand}>
        <section className={styles.finalCta} aria-labelledby="final-title">
          <span className={styles.eyebrow}>For your next trip</span>
          <h2 id="final-title">Everyone on the map.<br />Everyone in the loop.</h2>
          <CreateLink href={createHref} />
          <p>Road trips, bike rides, and meeting up across town.</p>
        </section>
      </PageBand>
      <PageBand>
        <footer className={styles.siteFooter}>
          <Brand />
          <span>Live route sharing for groups on the move.</span>
        </footer>
      </PageBand>
    </main>
  );
}
