import styles from './LoadingSkeleton.module.css';

export function LoadingSkeleton() {
  return (
    <div className={styles.container} role="status" aria-label="Loading">
      <div className={styles.row} />
    </div>
  );
}
