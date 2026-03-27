import type { AuthorityListItem } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { AuthoritySearchPort } from '../../domain/ports/authority-search-port';
import { useApplications } from './useApplications';
import { AuthoritySelector } from '../../components/AuthoritySelector/AuthoritySelector';
import { ApplicationCard } from '../../components/ApplicationCard/ApplicationCard';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import styles from './ApplicationsPage.module.css';

interface Props {
  browsePort: ApplicationsBrowsePort;
  searchPort: AuthoritySearchPort;
}

export function ApplicationsPage({ browsePort, searchPort }: Props) {
  const { selectedAuthority, applications, isLoading, error, selectAuthority } =
    useApplications(browsePort);

  function handleAuthoritySelect(authority: AuthorityListItem) {
    selectAuthority(authority);
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Applications</h1>

      <div className={styles.selectorWrapper}>
        <AuthoritySelector searchPort={searchPort} onSelect={handleAuthoritySelect} />
      </div>

      {isLoading && (
        <div className={styles.loading} aria-live="polite">Loading applications...</div>
      )}

      {error !== null && (
        <EmptyState
          title="Something went wrong"
          message={error.message}
          actionLabel="Try again"
          onAction={() => {
            if (selectedAuthority) {
              selectAuthority(selectedAuthority);
            }
          }}
        />
      )}

      {!isLoading && error === null && selectedAuthority === null && (
        <EmptyState
          icon="🏛️"
          title="Browse applications"
          message="Select an authority to browse planning applications."
        />
      )}

      {!isLoading && error === null && selectedAuthority !== null && applications.length === 0 && (
        <EmptyState
          icon="📋"
          title="No applications"
          message="No applications found for this authority."
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
