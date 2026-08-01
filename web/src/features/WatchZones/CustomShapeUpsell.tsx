import styles from './CustomShapeUpsell.module.css';

interface Props {
  onUpgradeClick?: () => void;
}

export function CustomShapeUpsell({ onUpgradeClick }: Props) {
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

      {onUpgradeClick && (
        <button type="button" className={styles.cta} onClick={onUpgradeClick}>
          Upgrade to draw custom shapes
        </button>
      )}
    </div>
  );
}
