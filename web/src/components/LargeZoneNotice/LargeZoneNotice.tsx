import styles from './LargeZoneNotice.module.css';

interface LoadingMessageProps {
  isVisible: boolean;
}

export function LargeZoneLoadingMessage({ isVisible }: LoadingMessageProps) {
  if (!isVisible) {
    return null;
  }

  return (
    <p className={styles.loadingMessage} role="status">
      Loading a large zone. This can take a few seconds.
    </p>
  );
}
