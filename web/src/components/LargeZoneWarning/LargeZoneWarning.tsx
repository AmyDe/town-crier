import styles from './LargeZoneWarning.module.css';

interface Props {
  isLarge: boolean;
}

export function LargeZoneWarning({ isLarge }: Props) {
  if (!isLarge) {
    return null;
  }

  return (
    <div className={styles.callout} role="status">
      <span className={styles.icon} aria-hidden="true">
        !
      </span>
      <div className={styles.copy}>
        <p className={styles.title}>Large zones are for notifications, not browsing</p>
        <p className={styles.body}>
          Good if you want to hear about everything in a wide area - tradespeople looking
          for leads, for example. Your notifications work as normal, but the map and list
          can take up to a minute to load the first time.
        </p>
        <p className={styles.body}>
          Just want to keep an eye on your street? Stick to 100-500 m in built-up areas,
          and under 2 km everywhere else.
        </p>
      </div>
    </div>
  );
}
