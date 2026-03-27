import { useState } from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar } from '../Sidebar/Sidebar';
import styles from './AppShell.module.css';

export function AppShell() {
  const [menuOpen, setMenuOpen] = useState(false);

  const handleMenuToggle = () => {
    setMenuOpen((prev) => !prev);
  };

  const handleOverlayClick = () => {
    setMenuOpen(false);
  };

  return (
    <div className={styles.shell}>
      <div
        className={`${styles.sidebarContainer} ${menuOpen ? styles.open : ''}`}
      >
        <Sidebar />
      </div>

      {menuOpen && (
        <div
          className={styles.overlay}
          data-testid="mobile-overlay"
          onClick={handleOverlayClick}
        />
      )}

      <div className={styles.main}>
        <header className={styles.mobileHeader}>
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
          <span className={styles.mobileTitle}>Town Crier</span>
        </header>

        <div className={styles.content}>
          <Outlet />
        </div>
      </div>
    </div>
  );
}
