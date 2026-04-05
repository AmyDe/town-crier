import styles from './BookmarkButton.module.css';

interface BookmarkButtonProps {
  isSaved: boolean;
  onToggle: () => void;
}

export function BookmarkButton({ isSaved, onToggle }: BookmarkButtonProps) {
  return (
    <button
      className={`${styles.button} ${isSaved ? styles.saved : ''}`}
      onClick={onToggle}
      aria-label={isSaved ? 'Unsave application' : 'Save application'}
      type="button"
    >
      <svg viewBox="0 0 24 24" width="18" height="18" xmlns="http://www.w3.org/2000/svg">
        {isSaved ? (
          <path d="M5 3h14v18l-7-4.5L5 21V3z" fill="currentColor" />
        ) : (
          <path d="M5 3h14v18l-7-4.5L5 21V3z" stroke="currentColor" strokeWidth="1.5" fill="none" />
        )}
      </svg>
    </button>
  );
}
