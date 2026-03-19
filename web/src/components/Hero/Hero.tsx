import styles from './Hero.module.css';

export function Hero() {
  return (
    <header className={styles.hero} role="banner">
      <h1 className={styles.headline}>
        Stay informed about what&apos;s being built in your neighbourhood
      </h1>
      <p className={styles.subheading}>
        Monitoring planning applications across 417 local authorities in England
        and Wales — so you don&apos;t have to.
      </p>
      <a
        className={styles.cta}
        href="https://apps.apple.com/app/town-crier"
        target="_blank"
        rel="noopener noreferrer"
      >
        Download on the App Store
      </a>
      <div className={styles.scrollIndicator} aria-label="Scroll down">
        &#8595;
      </div>
    </header>
  );
}
