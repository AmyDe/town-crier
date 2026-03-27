import styles from './Pagination.module.css';

interface Props {
  page: number;
  totalPages: number;
  onNext: () => void;
  onPrevious: () => void;
}

export function Pagination({ page, totalPages, onNext, onPrevious }: Props) {
  if (totalPages <= 1) {
    return null;
  }

  return (
    <nav className={styles.container} aria-label="Pagination">
      <button
        className={styles.button}
        onClick={onPrevious}
        disabled={page <= 1}
        aria-label="Previous page"
      >
        Previous
      </button>
      <span className={styles.label}>Page {page} of {totalPages}</span>
      <button
        className={styles.button}
        onClick={onNext}
        disabled={page >= totalPages}
        aria-label="Next page"
      >
        Next
      </button>
    </nav>
  );
}
