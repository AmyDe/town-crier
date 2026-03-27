import { Link } from 'react-router-dom';
import type { DashboardPort } from '../../domain/ports/dashboard-port';
import { useDashboard } from './useDashboard';
import { ApplicationCard } from '../../components/ApplicationCard/ApplicationCard';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import styles from './DashboardPage.module.css';

interface Props {
  port: DashboardPort;
}

function formatRadius(metres: number): string {
  if (metres >= 1000) {
    return `${(metres / 1000).toFixed(metres % 1000 === 0 ? 0 : 1)} km radius`;
  }
  return `${metres} m radius`;
}

export function DashboardPage({ port }: Props) {
  const { zones, recentApplications, isLoading, error } = useDashboard(port);

  if (isLoading) {
    return <div className={styles.loading}>Loading...</div>;
  }

  if (error !== null) {
    return <div className={styles.error}>{error}</div>;
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Dashboard</h1>

      {/* Quick links */}
      <section className={styles.section}>
        <h2 className={styles.sectionHeading}>Quick Links</h2>
        <nav className={styles.quickLinks}>
          <Link to="/saved" className={styles.quickLink}>
            Saved Applications
          </Link>
          <Link to="/notifications" className={styles.quickLink}>
            Notifications
          </Link>
        </nav>
      </section>

      {/* Watch zones */}
      <section className={styles.section}>
        <h2 className={styles.sectionHeading}>Watch Zones</h2>
        {zones.length === 0 ? (
          <EmptyState
            icon="📍"
            title="No watch zones yet"
            message="Create a watch zone to start monitoring planning applications in your area."
            actionLabel="Add Watch Zone"
            onAction={() => {
              window.location.href = '/watch-zones';
            }}
          />
        ) : (
          <div className={styles.zonesGrid}>
            {zones.map(zone => (
              <article key={zone.id} className={styles.zoneCard}>
                <h3 className={styles.zoneName}>{zone.name}</h3>
                <p className={styles.zoneDetail}>{formatRadius(zone.radiusMetres)}</p>
              </article>
            ))}
          </div>
        )}
      </section>

      {/* Recent applications */}
      {recentApplications.length > 0 && (
        <section className={styles.section}>
          <h2 className={styles.sectionHeading}>Recent Applications</h2>
          <ul className={styles.applicationsList}>
            {recentApplications.map(app => (
              <li key={app.uid}>
                <ApplicationCard application={app} />
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
