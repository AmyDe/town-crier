import styles from './FullPageLoader.module.css';

interface FullPageLoaderProps {
  message?: string;
}

export function FullPageLoader({ message = 'Loading…' }: FullPageLoaderProps) {
  return (
    <div className={styles.container} role="status" aria-label={message}>
      <div className={styles.spinner} />
      <p className={styles.message}>{message}</p>
    </div>
  );
}
