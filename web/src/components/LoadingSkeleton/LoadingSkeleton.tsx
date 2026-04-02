import styles from './LoadingSkeleton.module.css';

type RowWidth = 'widthNarrow' | 'widthMedium' | 'widthWide';

const SKELETON_ROWS: readonly RowWidth[] = [
  'widthMedium',
  'widthWide',
  'widthNarrow',
  'widthWide',
  'widthMedium',
];

export function LoadingSkeleton() {
  return (
    <div className={styles.container} role="status" aria-label="Loading">
      <div className={styles.header} data-testid="skeleton-row" />
      {SKELETON_ROWS.map((widthClass, index) => (
        <div
          key={index}
          className={`${styles.row} ${styles[widthClass]}`}
          data-testid="skeleton-row"
        />
      ))}
    </div>
  );
}
