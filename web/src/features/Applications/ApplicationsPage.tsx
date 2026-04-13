import { useNavigate } from 'react-router-dom';
import type { WatchZoneSummary } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import { useApplications } from './useApplications';
import { ApplicationCard } from '../../components/ApplicationCard/ApplicationCard';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import { useFetchData } from '../../hooks/useFetchData';
import styles from './ApplicationsPage.module.css';

interface ZonesPort {
  fetchZones(): Promise<readonly WatchZoneSummary[]>;
}

interface Props {
  zonesPort: ZonesPort;
  browsePort: ApplicationsBrowsePort;
}

export function ApplicationsPage({ zonesPort, browsePort }: Props) {
  const navigate = useNavigate();
  const { data: zones, isLoading: isLoadingZones, error: zonesError } =
    useFetchData(() => zonesPort.fetchZones(), [zonesPort]);
  const { selectedZone, applications, isLoading: isLoadingApps, error: appsError, selectZone } =
    useApplications(browsePort);

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Applications</h1>

      {selectedZone !== null && (
        <nav className={styles.breadcrumb} aria-label="Breadcrumb">
          <button className={styles.breadcrumbLink} onClick={() => selectZone(null)}>
            Watch Zones
          </button>
          <span aria-hidden="true">&rsaquo;</span>
          <span className={styles.breadcrumbCurrent}>{selectedZone.name}</span>
        </nav>
      )}

      {selectedZone === null && (
        <>
          {isLoadingZones && (
            <div className={styles.loading} aria-live="polite">Loading zones...</div>
          )}

          {zonesError !== null && (
            <EmptyState title="Something went wrong" message={zonesError} />
          )}

          {!isLoadingZones && zonesError === null && (zones ?? []).length === 0 && (
            <EmptyState
              icon="📍"
              title="No watch zones yet"
              message="Set up a watch zone to start browsing applications."
              actionLabel="Create watch zone"
              onAction={() => navigate('/watch-zones/new')}
            />
          )}

          {!isLoadingZones && zonesError === null && (zones ?? []).length > 0 && (
            <div className={styles.authorityGrid}>
              {(zones ?? []).map((zone) => (
                <button
                  key={zone.id}
                  className={styles.authorityCard}
                  onClick={() => selectZone(zone)}
                >
                  <span className={styles.authorityName}>{zone.name}</span>
                </button>
              ))}
            </div>
          )}
        </>
      )}

      {selectedZone !== null && (
        <>
          {isLoadingApps && (
            <div className={styles.loading} aria-live="polite">Loading applications...</div>
          )}

          {appsError !== null && (
            <EmptyState
              title="Something went wrong"
              message={appsError}
              actionLabel="Try again"
              onAction={() => selectZone(selectedZone)}
            />
          )}

          {!isLoadingApps && appsError === null && applications.length === 0 && (
            <EmptyState
              icon="📋"
              title="No applications"
              message="No applications found in this zone."
            />
          )}

          {!isLoadingApps && appsError === null && applications.length > 0 && (
            <ul className={styles.list}>
              {applications.map((app) => (
                <li key={app.uid}>
                  <ApplicationCard application={app} />
                </li>
              ))}
            </ul>
          )}
        </>
      )}
    </div>
  );
}
