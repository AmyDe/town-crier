import styles from './Footer.module.css';

export function Footer() {
  const currentYear = new Date().getFullYear();

  return (
    <footer id="footer" className={styles.footer}>
      <div className={styles.container}>
        <div className={styles.cta}>
          <h2 className={styles.ctaHeading}>
            Your neighbourhood is changing. Stay informed.
          </h2>
          <a
            href="https://apps.apple.com/app/town-crier"
            className={styles.downloadButton}
            aria-label="Download on the App Store"
          >
            Download on the App Store
          </a>
        </div>

        <div className={styles.bottom}>
          <p className={styles.copyright}>
            © {currentYear} Town Crier
          </p>

          <nav aria-label="Legal" className={styles.legal}>
            <a href="/privacy" className={styles.legalLink}>Privacy Policy</a>
            <a href="/terms" className={styles.legalLink}>Terms of Service</a>
          </nav>
        </div>
      </div>
    </footer>
  );
}
