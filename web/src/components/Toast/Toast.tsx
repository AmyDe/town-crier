import { useEffect } from 'react';
import styles from './Toast.module.css';

interface Props {
  message: string;
  onDismiss: () => void;
  duration?: number;
}

export function Toast({ message, onDismiss, duration = 4000 }: Props) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, duration);
    return () => clearTimeout(timer);
  }, [onDismiss, duration]);

  return (
    <div className={styles.toast} role="status">
      {message}
    </div>
  );
}
