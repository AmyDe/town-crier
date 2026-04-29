import { useNavigate } from 'react-router-dom';
import type { WatchZoneSummary, ApplicationStatus } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
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
  savedRepository: SavedApplicationRepository;
}

const ALL_ZONES_VALUE = '__all__';

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

export function ApplicationsPage({ zonesPort, browsePort, savedRepository }: Props) {
  const navigate = useNavigate();
  const {
    data: zones,
    isLoading: isLoadingZones,
    error: zonesError,
  } = useFetchData(() => zonesPort.fetchZones(), [zonesPort]);

  const {
    selectedZone,
    isAllZonesSelected,
    applications,
    isLoading: isLoadingApps,
    error: appsError,
    selectedStatusFilter,
    isSavedFilterActive,
    selectZone,
    selectAllZones,
    setStatusFilter,
    activateSavedFilter,
    deactivateSavedFilter,
  } = useApplications({
    browsePort,
    savedRepository,
    zones: zones ?? [],
  });

  const hasZones = (zones ?? []).length > 0;

  function handleZoneChange(value: string) {
    if (value === ALL_ZONES_VALUE) {
      selectAllZones();
      return;
    }
    const zone = (zones ?? []).find((z) => z.id === value);
    if (zone) {
      selectZone(zone);
    }
  }

  function handleStatusClick(status: ApplicationStatus | null) {
    setStatusFilter(status);
  }

  function handleSavedToggle() {
    if (isSavedFilterActive) {
      deactivateSavedFilter();
    } else {
      void activateSavedFilter();
    }
  }

  const showAllZonesEmptyHint = isAllZonesSelected && !isSavedFilterActive;

  // Determine the current value for the zone selector
  const zoneSelectorValue = isAllZonesSelected
    ? ALL_ZONES_VALUE
    : (selectedZone?.id ?? '');

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
                <option value={ALL_ZONES_VALUE}>All</option>
                {(zones ?? []).map((zone) => (
                  <option key={zone.id} value={zone.id}>
                    {zone.name}
                  </option>
                ))}
              </select>
            </label>

            <div className={styles.statusChips} role="group" aria-label="Status filter">
              {STATUS_CHIPS.map((chip) => {
                const isPressed =
                  chip.status === null
                    ? selectedStatusFilter === null && !isSavedFilterActive
                    : selectedStatusFilter === chip.status;
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

            <button
              type="button"
              role="switch"
              aria-checked={isSavedFilterActive}
              aria-label="Saved"
              className={`${styles.savedToggle} ${isSavedFilterActive ? styles.savedToggleOn : ''}`}
              onClick={handleSavedToggle}
            >
              Saved
            </button>
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

          {!isLoadingApps && appsError === null && showAllZonesEmptyHint && (
            <EmptyState
              icon="📌"
              message="Pick a zone to see applications, or turn on Saved to see everything you've bookmarked."
            />
          )}

          {!isLoadingApps &&
            appsError === null &&
            !showAllZonesEmptyHint &&
            applications.length === 0 && (
              <EmptyState
                icon="📋"
                title="No applications"
                message={
                  isSavedFilterActive
                    ? "You haven't saved any applications matching this view."
                    : 'No applications found in this zone.'
                }
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
