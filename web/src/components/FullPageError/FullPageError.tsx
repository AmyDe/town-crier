import styles from './FullPageError.module.css';

interface Props {
  message: string;
  onRetry: () => void;
  onSignOut: () => void;
}

export function FullPageError({ message, onRetry, onSignOut }: Props) {
  return (
    <div className={styles.container} role="alert">
      <h1 className={styles.title}>Something went wrong</h1>
      <p className={styles.message}>{message}</p>
      <div className={styles.actions}>
        <button className={styles.retryButton} onClick={onRetry}>
          Try again
        </button>
        <button className={styles.signOutButton} onClick={onSignOut}>
          Sign out
        </button>
      </div>
    </div>
  );
}
