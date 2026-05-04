import styles from './LargeRadiusWarning.module.css';

/**
 * Radius (in metres) at or above which the LargeRadiusWarning callout is
 * shown. Mirrors the iOS sibling (tc-1zb7) — 2 km is the upper edge of the
 * "small zone" range we recommend, so any selection at or above warrants the
 * heads-up.
 */
export const LARGE_RADIUS_THRESHOLD_METRES = 2000;

interface Props {
  radiusMetres: number;
}

export function LargeRadiusWarning({ radiusMetres }: Props) {
  if (radiusMetres < LARGE_RADIUS_THRESHOLD_METRES) {
    return null;
  }

  return (
    <div className={styles.callout} role="status">
      <span className={styles.icon} aria-hidden="true">
        !
      </span>
      <div className={styles.copy}>
        <p className={styles.title}>Heads up — large zones get noisy</p>
        <p className={styles.body}>
          A wide watch zone can produce hundreds of notifications a day, especially in
          cities. We recommend 100–500m in built-up areas, and under 2km everywhere
          else.
        </p>
      </div>
    </div>
  );
}
