import { NavLink } from 'react-router-dom';
import styles from './Sidebar.module.css';

const NAV_ITEMS = [
  { label: 'Dashboard', to: '/dashboard' },
  { label: 'Applications', to: '/applications' },
  { label: 'Saved', to: '/saved' },
  { label: 'Watch Zones', to: '/watch-zones' },
  { label: 'Map', to: '/map' },
  { label: 'Settings', to: '/settings' },
] as const;

export function Sidebar() {
  return (
    <nav className={styles.sidebar} aria-label="Main">
      <NavLink to="/" className={styles.appName}>
        Town Crier
      </NavLink>
      <ul className={styles.navList}>
        {NAV_ITEMS.map((item) => (
          <li key={item.to}>
            <NavLink
              to={item.to}
              className={({ isActive }) =>
                `${styles.navLink} ${isActive ? styles.active : ''}`
              }
            >
              {item.label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}
