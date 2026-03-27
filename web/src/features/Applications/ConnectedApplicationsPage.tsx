import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { applicationsApi } from '../../api/applications';
import { authoritiesApi } from '../../api/authorities';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { AuthoritySearchPort } from '../../domain/ports/authority-search-port';
import { ApplicationsPage } from './ApplicationsPage';

export function ConnectedApplicationsPage() {
  const client = useApiClient();

  const browsePort: ApplicationsBrowsePort = useMemo(
    () => ({
      fetchByAuthority: (authorityId) =>
        applicationsApi(client).getByAuthority(authorityId),
    }),
    [client],
  );

  const searchPort: AuthoritySearchPort = useMemo(
    () => ({
      search: (query) => authoritiesApi(client).list(query),
    }),
    [client],
  );

  return <ApplicationsPage browsePort={browsePort} searchPort={searchPort} />;
}
