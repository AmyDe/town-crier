import styles from './ProGate.module.css';

interface Props {
  featureName: string;
}

export function ProGate({ featureName }: Props) {
  return (
    <div className={styles.container} role="status">
      <span className={styles.icon} aria-hidden="true">&#x2B50;</span>
      <h2 className={styles.title}>Pro Feature</h2>
      <p className={styles.message}>
        <span className={styles.highlight}>{featureName}</span> is available on
        the Pro plan. Upgrade in the iOS app to unlock this feature.
      </p>
    </div>
  );
}
