import styles from './EmptyState.module.css';

interface Props {
  message: string;
  title?: string;
  icon?: string;
  actionLabel?: string;
  onAction?: () => void;
}

export function EmptyState({ message, title, icon, actionLabel, onAction }: Props) {
  return (
    <div className={styles.container}>
      {icon !== undefined && (
        <span className={styles.icon} aria-hidden="true">{icon}</span>
      )}
      {title !== undefined && (
        <h2 className={styles.title}>{title}</h2>
      )}
      <p className={styles.message}>{message}</p>
      {actionLabel !== undefined && onAction !== undefined && (
        <button className={styles.action} onClick={onAction}>
          {actionLabel}
        </button>
      )}
    </div>
  );
}
