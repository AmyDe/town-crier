import { useNavigate } from 'react-router-dom';
import type { AuthorityListItem } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { UserAuthoritiesPort } from '../../domain/ports/user-authorities-port';
import { useUserAuthorities } from './useUserAuthorities';
import { useApplications } from './useApplications';
import { ApplicationCard } from '../../components/ApplicationCard/ApplicationCard';
import { EmptyState } from '../../components/EmptyState/EmptyState';
import styles from './ApplicationsPage.module.css';

interface Props {
  userAuthoritiesPort: UserAuthoritiesPort;
  browsePort: ApplicationsBrowsePort;
}

export function ApplicationsPage({ userAuthoritiesPort, browsePort }: Props) {
  const navigate = useNavigate();
  const { authorities, isLoading: isLoadingAuthorities, error: authoritiesError } =
    useUserAuthorities(userAuthoritiesPort);
  const { selectedAuthority, applications, isLoading: isLoadingApps, error: appsError, selectAuthority } =
    useApplications(browsePort);

  function handleAuthorityClick(authority: AuthorityListItem) {
    selectAuthority(authority);
  }

  function handleBackToAuthorities() {
    selectAuthority(null);
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Applications</h1>

      {selectedAuthority !== null && (
        <nav className={styles.breadcrumb} aria-label="Breadcrumb">
          <button
            className={styles.breadcrumbLink}
            onClick={handleBackToAuthorities}
          >
            Authorities
          </button>
          <span aria-hidden="true">&rsaquo;</span>
          <span className={styles.breadcrumbCurrent}>{selectedAuthority.name}</span>
        </nav>
      )}

      {selectedAuthority === null && (
        <>
          {isLoadingAuthorities && (
            <div className={styles.loading} aria-live="polite">Loading authorities...</div>
          )}

          {authoritiesError !== null && (
            <EmptyState
              title="Something went wrong"
              message={authoritiesError.message}
            />
          )}

          {!isLoadingAuthorities && authoritiesError === null && authorities.length === 0 && (
            <EmptyState
              icon="🏛️"
              title="No watch zones yet"
              message="Set up a watch zone to start browsing applications."
              actionLabel="Create watch zone"
              onAction={() => navigate('/watch-zones/new')}
            />
          )}

          {!isLoadingAuthorities && authoritiesError === null && authorities.length > 0 && (
            <div className={styles.authorityGrid}>
              {authorities.map((authority) => (
                <button
                  key={authority.id}
                  className={styles.authorityCard}
                  onClick={() => handleAuthorityClick(authority)}
                >
                  <span className={styles.authorityName}>{authority.name}</span>
                  <span className={styles.authorityType}>{authority.areaType}</span>
                </button>
              ))}
            </div>
          )}
        </>
      )}

      {selectedAuthority !== null && (
        <>
          {isLoadingApps && (
            <div className={styles.loading} aria-live="polite">Loading applications...</div>
          )}

          {appsError !== null && (
            <EmptyState
              title="Something went wrong"
              message={appsError}
              actionLabel="Try again"
              onAction={() => selectAuthority(selectedAuthority)}
            />
          )}

          {!isLoadingApps && appsError === null && applications.length === 0 && (
            <EmptyState
              icon="📋"
              title="No applications"
              message="No applications found for this authority."
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
