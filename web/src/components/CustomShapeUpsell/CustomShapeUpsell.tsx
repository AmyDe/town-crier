import styles from './CustomShapeUpsell.module.css';

interface Props {
  /**
   * App Store link (campaign-tagged, see `appStoreUrl` in `config/links.ts`)
   * for the upgrade CTA. Web has no in-app checkout — subscriptions are an
   * iOS in-app purchase — so this points out to the store listing, mirroring
   * `Pricing.tsx`. No CTA renders when omitted.
   */
  upgradeHref?: string;
}

export function CustomShapeUpsell({ upgradeHref }: Props) {
  return (
    <div className={styles.container}>
      <svg
        className={styles.graphic}
        viewBox="0 0 200 140"
        aria-hidden="true"
        focusable="false"
      >
        <circle
          className={styles.circleOutline}
          cx="55"
          cy="70"
          r="48"
        />
        <path
          className={styles.polygonShape}
          d="M118 30 L162 42 L178 78 L156 112 L108 118 L96 82 L112 58 Z"
        />
      </svg>

      <h2 className={styles.heading}>Draw any shape, not just a circle</h2>
      <p className={styles.body}>
        Trace the exact area you care about, a street, an estate, an odd-shaped site.
        We&apos;ll watch it and notify you the moment something changes.
      </p>

      {upgradeHref && (
        <a
          className={styles.cta}
          href={upgradeHref}
          target="_blank"
          rel="noopener noreferrer"
        >
          Upgrade to draw custom shapes
        </a>
      )}
    </div>
  );
}
