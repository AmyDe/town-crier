import { appStoreUrl } from '../../config/links';
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
            href={appStoreUrl('web-home')}
            className={styles.downloadButton}
            aria-label="Download on the App Store"
            target="_blank"
            rel="noopener noreferrer"
          >
            Download on the App Store
          </a>
        </div>

        <nav aria-label="Explore" className={styles.explore}>
          <a href="/planning/" className={styles.exploreLink}>Planning applications by council</a>
          <a href="/planning/towns/" className={styles.exploreLink}>Planning applications by town</a>
        </nav>

        <div className={styles.bottom}>
          <p className={styles.copyright}>
            © {currentYear} Town Crier
          </p>

          <nav aria-label="Legal" className={styles.legal}>
            <a href="/legal/privacy" className={styles.legalLink}>Privacy Policy</a>
            <a href="/legal/terms" className={styles.legalLink}>Terms of Service</a>
          </nav>
        </div>

        <p className={styles.companyInfo}>
          Ivo and the Bea Ltd · Registered in England &amp; Wales · Company No. 17222369
        </p>
      </div>
    </footer>
  );
}
