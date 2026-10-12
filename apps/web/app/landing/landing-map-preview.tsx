import styles from "./landing.module.css";

// Illustration only. This preview never requests or displays visitor location.
export function LandingMapPreview() {
  return (
    <div className={styles.mapDemo} id="landing-map">
      <div className={styles.mapTop}>
        <span className={styles.liveDot} />
        <span>
          Weekend escape
        </span>
        <span className={styles.sample}>
          Demo route
        </span>
      </div>
      <svg className={styles.cityMap} viewBox="0 0 700 510" role="img" aria-labelledby="landing-map-title">
        <title id="landing-map-title">
          Illustrative Ljubljana map showing three friends sharing their positions and traveled paths
        </title>
        <defs>
          <pattern id="landing-blocks" width="88" height="75" patternTransform="rotate(-18)" patternUnits="userSpaceOnUse">
            <rect x="9" y="9" width="65" height="51" rx="7" fill="var(--map-block)" />
            <path d="M0 0H88M0 0V75" stroke="var(--map-street)" strokeWidth="6" />
          </pattern>
        </defs>
        <rect width="700" height="510" fill="var(--map-base)" />
        <rect width="700" height="510" fill="url(#landing-blocks)" />
        <path id="landing-park" d="M0 20L100 0 195 33 180 147 89 195 0 151Z" fill="var(--map-park)">
          <title>
            Tivoli park
          </title>
        </path>
        <path d="M525 382L641 363 700 400V510H486Z" fill="var(--map-park)" />
        <path id="landing-river" d="M405 -30C285 60 474 144 373 238S450 349 390 416 459 502 439 550" fill="none" stroke="var(--map-river)" strokeWidth="27">
          <title>
            River
          </title>
        </path>
        <g fill="none" stroke="var(--map-avenue)" strokeWidth="10">
          <path d="M-20 325L155 267 248 211 432 190 728 77" />
          <path d="M99 -10L216 121 248 211 282 353 245 530" />
          <path d="M-10 445L282 353 407 346 716 282" />
        </g>
        <g fontFamily="system-ui,sans-serif" fontSize="12" fill="var(--map-label)" letterSpacing="2">
          <text x="69" y="90">
            TIVOLI
          </text>
          <text x="465" y="144">
            CENTER
          </text>
          <text x="102" y="419">
            VIČ
          </text>
          <text x="503" y="439">
            TRNOVO
          </text>
        </g>
        <g fill="none" strokeLinecap="round" strokeLinejoin="round">
          <path id="landing-route-a" className={styles.routeLine} d="M135 330L209 294 248 211 348 202 404 213" stroke="#22c55e" strokeWidth="4">
            <title>
              Alex&#x27;s traveled path
            </title>
          </path>
          <path id="landing-route-b" className={[styles.routeLine, styles.delayed].join(" ")} d="M516 334L407 346 352 295 323 232" stroke="#a99bfb" strokeWidth="4">
            <title>
              Mia&#x27;s traveled path
            </title>
          </path>
          <path id="landing-route-c" className={[styles.routeLine, styles.delayed].join(" ")} d="M170 131L216 121 244 187" stroke="#f6bf65" strokeWidth="4">
            <title>
              Sam&#x27;s traveled path
            </title>
          </path>
        </g>
        <g id="landing-alex" transform="translate(404 213)">
          <title>
            Alex, sharing location
          </title>
          <circle className={styles.pulse} r="23" fill="#22c55e" opacity=".12" />
          <circle r="10" fill="#22c55e" stroke="#f8fafc" strokeWidth="3" />
          <rect x="-27" y="-49" width="54" height="26" rx="9" fill="var(--bg-elevated)" stroke="var(--border)" />
          <text y="-32" textAnchor="middle" fill="var(--text)" fontSize="11" fontWeight="650">
            Alex
          </text>
        </g>
        <g id="landing-mia" transform="translate(323 232)">
          <title>
            Mia, sharing location
          </title>
          <circle r="9" fill="#a99bfb" stroke="#f8fafc" strokeWidth="3" />
          <rect x="-26" y="20" width="52" height="26" rx="9" fill="var(--bg-elevated)" stroke="var(--border)" />
          <text y="37" textAnchor="middle" fill="var(--text)" fontSize="11" fontWeight="650">
            Mia
          </text>
        </g>
        <g id="landing-sam" transform="translate(244 187)">
          <title>
            Sam, sharing location
          </title>
          <circle r="9" fill="#f6bf65" stroke="#f8fafc" strokeWidth="3" />
          <rect x="-27" y="-49" width="54" height="26" rx="9" fill="var(--bg-elevated)" stroke="var(--border)" />
          <text y="-32" textAnchor="middle" fill="var(--text)" fontSize="11" fontWeight="650">
            Sam
          </text>
        </g>
      </svg>
      <div className={styles.mapCaption}>
        Illustrative map · Ljubljana
      </div>
      <div className={styles.groupCard}>
        <div className={styles.groupHeading}>
          <strong>
            Your group, on one map
          </strong>
          <span className={styles.liveLabel}>
            3 sharing
          </span>
        </div>
        <div className={styles.people}>
          <span>
            <i className={[styles.avatar, styles.green].join(" ")}>
              A
            </i>
             Alex
            <small>
              Car
            </small>
          </span>
          <span>
            <i className={[styles.avatar, styles.purple].join(" ")}>
              M
            </i>
             Mia
            <small>
              Bicycle
            </small>
          </span>
          <span>
            <i className={[styles.avatar, styles.amber].join(" ")}>
              S
            </i>
             Sam
            <small>
              Walking
            </small>
          </span>
        </div>
      </div>
    </div>
  );
}
