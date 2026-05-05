import { useNavigate } from 'react-router-dom';
import type { WatchZoneSummary, ApplicationStatus } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { NotificationStateRepository } from '../../domain/ports/notification-state-repository';
import { useApplications, type ApplicationsSort } from './useApplications';
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
  notificationStateRepository: NotificationStateRepository;
}

interface StatusChip {
  readonly label: string;
  readonly status: ApplicationStatus | null;
}

const STATUS_CHIPS: readonly StatusChip[] = [
  { label: 'All', status: null },
  { label: 'Pending', status: 'Undecided' },
  { label: 'Granted', status: 'Permitted' },
  { label: 'Granted with conditions', status: 'Conditions' },
  { label: 'Refused', status: 'Rejected' },
  { label: 'Withdrawn', status: 'Withdrawn' },
  { label: 'Appealed', status: 'Appealed' },
];

const SORT_OPTION_LABELS: Record<ApplicationsSort, string> = {
  'recent-activity': 'Recent activity',
  newest: 'Newest',
  oldest: 'Oldest',
  status: 'Status',
  distance: 'Distance',
};

export function ApplicationsPage({
  zonesPort,
  browsePort,
  notificationStateRepository,
}: Props) {
  const navigate = useNavigate();
  const {
    data: zones,
    isLoading: isLoadingZones,
    error: zonesError,
  } = useFetchData(() => zonesPort.fetchZones(), [zonesPort]);

  const {
    selectedZone,
    applications,
    isLoading: isLoadingApps,
    error: appsError,
    selectedStatusFilter,
    unreadOnly,
    unreadCount,
    sort,
    availableSortOptions,
    selectZone,
    setStatusFilter,
    setUnreadOnly,
    setSort,
    markAllRead,
  } = useApplications({
    browsePort,
    zones: zones ?? [],
    notificationStateRepository,
  });

  const hasZones = (zones ?? []).length > 0;
  const hasUnread = unreadCount > 0;

  function handleZoneChange(value: string) {
    const zone = (zones ?? []).find((z) => z.id === value);
    if (zone) {
      selectZone(zone);
    }
  }

  function handleStatusClick(status: ApplicationStatus | null) {
    setStatusFilter(status);
  }

  function handleUnreadClick() {
    setUnreadOnly(!unreadOnly);
  }

  function handleSortChange(value: string) {
    if (isApplicationsSort(value)) {
      setSort(value);
    }
  }

  async function handleMarkAllRead() {
    await markAllRead();
  }

  const zoneSelectorValue = selectedZone?.id ?? '';

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Applications</h1>

      {isLoadingZones && (
        <div className={styles.loading} aria-live="polite">Loading zones...</div>
      )}

      {zonesError !== null && (
        <EmptyState title="Something went wrong" message={zonesError} />
      )}

      {!isLoadingZones && zonesError === null && !hasZones && (
        <EmptyState
          icon="📍"
          title="No watch zones yet"
          message="Set up a watch zone to start browsing applications."
          actionLabel="Create watch zone"
          onAction={() => navigate('/watch-zones/new')}
        />
      )}

      {!isLoadingZones && zonesError === null && hasZones && (
        <>
          <div className={styles.filterBar} role="toolbar" aria-label="Filters">
            <label className={styles.zoneSelectorLabel}>
              <span className={styles.srOnly}>Zone</span>
              <select
                className={styles.zoneSelector}
                aria-label="Zone"
                value={zoneSelectorValue}
                onChange={(e) => handleZoneChange(e.target.value)}
              >
                {(zones ?? []).map((zone) => (
                  <option key={zone.id} value={zone.id}>
                    {zone.name}
                  </option>
                ))}
              </select>
            </label>

            <div className={styles.statusChips} role="group" aria-label="Status filter">
              {hasUnread && (
                <button
                  type="button"
                  className={`${styles.chip} ${unreadOnly ? styles.chipPressed : ''}`}
                  aria-pressed={unreadOnly}
                  onClick={handleUnreadClick}
                >
                  Unread ({unreadCount})
                </button>
              )}
              {STATUS_CHIPS.map((chip) => {
                const isPressed =
                  !unreadOnly && selectedStatusFilter === chip.status;
                return (
                  <button
                    key={chip.label}
                    type="button"
                    className={`${styles.chip} ${isPressed ? styles.chipPressed : ''}`}
                    aria-pressed={isPressed}
                    onClick={() => handleStatusClick(chip.status)}
                  >
                    {chip.label}
                  </button>
                );
              })}
            </div>

            <label className={styles.sortLabel}>
              <span className={styles.srOnly}>Sort</span>
              <select
                className={styles.sortSelector}
                aria-label="Sort"
                value={sort}
                onChange={(e) => handleSortChange(e.target.value)}
              >
                {availableSortOptions.map((option) => (
                  <option key={option} value={option}>
                    {SORT_OPTION_LABELS[option]}
                  </option>
                ))}
              </select>
            </label>

            {hasUnread && (
              <button
                type="button"
                className={styles.markAllReadButton}
                onClick={handleMarkAllRead}
              >
                Mark all read
              </button>
            )}
          </div>

          {isLoadingApps && (
            <div className={styles.loading} aria-live="polite">Loading applications...</div>
          )}

          {appsError !== null && (
            <EmptyState
              title="Something went wrong"
              message={appsError}
              actionLabel="Try again"
              onAction={() => selectedZone && selectZone(selectedZone)}
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

function isApplicationsSort(value: string): value is ApplicationsSort {
  return (
    value === 'recent-activity' ||
    value === 'newest' ||
    value === 'oldest' ||
    value === 'status' ||
    value === 'distance'
  );
}
