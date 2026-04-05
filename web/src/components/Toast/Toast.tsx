import styles from './Toast.module.css';

interface Props {
  message: string;
  onDismiss: () => void;
  duration?: number;
}

export function Toast({ message }: Props) {
  return (
    <div className={styles.toast} role="status">
      {message}
    </div>
  );
}
