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
        <p className={styles.title}>Large zones can be slower to browse</p>
        <p className={styles.body}>
          Your notifications work as normal, but the map and list can take a few seconds
          to load, especially in cities.
        </p>
        <p className={styles.body}>
          Just want to keep an eye on your street? Stick to 100-500 m in built-up areas,
          and under 2 km everywhere else.
        </p>
      </div>
    </div>
  );
}
