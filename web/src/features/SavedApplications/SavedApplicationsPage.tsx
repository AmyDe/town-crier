import type { ApplicationStatus } from '../../domain/types';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
import { useSavedApplications } from './useSavedApplications';
import { ApplicationCard } from '../../components/ApplicationCard/ApplicationCard';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import styles from './SavedApplicationsPage.module.css';

interface Props {
  savedRepository: SavedApplicationRepository;
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

export function SavedApplicationsPage({ savedRepository }: Props) {
  const {
    applications,
    isLoading,
    error,
    selectedStatusFilter,
    setStatusFilter,
  } = useSavedApplications({ savedRepository });

  function handleStatusClick(status: ApplicationStatus | null) {
    setStatusFilter(status);
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Saved</h1>

      <div className={styles.filterBar} role="toolbar" aria-label="Filters">
        <div className={styles.statusChips} role="group" aria-label="Status filter">
          {STATUS_CHIPS.map((chip) => {
            const isPressed = selectedStatusFilter === chip.status;
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
      </div>

      {isLoading && (
        <div className={styles.loading} aria-live="polite">
          Loading saved applications...
        </div>
      )}

      {error !== null && (
        <EmptyState title="Something went wrong" message={error} />
      )}

      {!isLoading && error === null && applications.length === 0 && (
        <EmptyState
          icon="🔖"
          title={
            selectedStatusFilter === null
              ? 'No saved applications yet'
              : 'No matches'
          }
          message={
            selectedStatusFilter === null
              ? 'Bookmark applications you want to track. Tap the bookmark icon on any application detail.'
              : 'No saved applications match this filter.'
          }
        />
      )}

      {!isLoading && error === null && applications.length > 0 && (
        <ul className={styles.list}>
          {applications.map((app) => (
            <li key={app.uid}>
              <ApplicationCard application={app} />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
