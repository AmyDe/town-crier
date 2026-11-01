import styles from './LargeZoneNotice.module.css';

interface NoticeProps {
  isLarge: boolean;
}

export function LargeZoneNotice({ isLarge }: NoticeProps) {
  if (!isLarge) {
    return null;
  }

  return (
    <p className={styles.notice}>
      <span className={styles.icon} aria-hidden="true">
        !
      </span>
      Large zone: built for notifications. This can take up to a minute to load the first
      time.
    </p>
  );
}

interface LoadingMessageProps {
  isVisible: boolean;
}

export function LargeZoneLoadingMessage({ isVisible }: LoadingMessageProps) {
  if (!isVisible) {
    return null;
  }

  return (
    <p className={styles.loadingMessage} role="status">
      Loading a large zone. This can take up to a minute the first time.
    </p>
  );
}
