import { useState } from 'react';
import { appStoreUrl } from '../../config/links';
import { useTheme } from '../../hooks/useTheme';
import { useNavbarAuth } from './useNavbarAuth';
import { ThemeToggle } from '../ThemeToggle/ThemeToggle';
import styles from './Navbar.module.css';

const NAV_LINKS = [
  { label: 'Features', href: '#how-it-works' },
  { label: 'Pricing', href: '#pricing' },
  { label: 'FAQ', href: '#faq' },
] as const;

export function Navbar() {
  const { theme, toggleTheme } = useTheme();
  const { isAuthenticated, handleSignIn } = useNavbarAuth();
  const [menuOpen, setMenuOpen] = useState(false);

  const handleMenuToggle = () => {
    setMenuOpen((prev) => !prev);
  };

  return (
    <nav className={styles.nav}>
      <div className={styles.inner}>
        <a href="#" className={styles.logo}>
          Town Crier
        </a>

        <div
          className={styles.links}
          data-testid="nav-links"
          data-open={menuOpen ? 'true' : 'false'}
        >
          {NAV_LINKS.map((link) => (
            <a key={link.href} href={link.href} className={styles.link}>
              {link.label}
            </a>
          ))}
        </div>

        <div className={styles.actions}>
          <ThemeToggle theme={theme} toggleTheme={toggleTheme} />

          {isAuthenticated ? (
            <a href="/dashboard" className={styles.signIn}>
              Sign In
            </a>
          ) : (
            <button
              className={styles.signIn}
              onClick={() => void handleSignIn?.()}
            >
              Sign In
            </button>
          )}

          <a
            href={appStoreUrl('web-home')}
            className={styles.cta}
            target="_blank"
            rel="noopener noreferrer"
          >
            Download
          </a>

          <button
            className={styles.hamburger}
            onClick={handleMenuToggle}
            aria-label="Menu"
            aria-expanded={menuOpen ? 'true' : 'false'}
          >
            <svg
              width="24"
              height="24"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              {menuOpen ? (
                <>
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </>
              ) : (
                <>
                  <line x1="3" y1="6" x2="21" y2="6" />
                  <line x1="3" y1="12" x2="21" y2="12" />
                  <line x1="3" y1="18" x2="21" y2="18" />
                </>
              )}
            </svg>
          </button>
        </div>
      </div>

      {/* Masthead double rule (Public Notice component language) —
          2.5px heavy rule over a 1px hairline, landing page only. */}
      <div className={styles.ruleHeavy} data-testid="masthead-rule-heavy" />
      <div className={styles.ruleHairline} data-testid="masthead-rule-hairline" />
    </nav>
  );
}
