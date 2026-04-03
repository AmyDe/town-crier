import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { applicationsApi } from '../../api/applications';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { UserAuthoritiesPort } from '../../domain/ports/user-authorities-port';
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

  const userAuthoritiesPort: UserAuthoritiesPort = useMemo(
    () => ({
      fetchMyAuthorities: () => applicationsApi(client).getMyAuthorities(),
    }),
    [client],
  );

  return <ApplicationsPage userAuthoritiesPort={userAuthoritiesPort} browsePort={browsePort} />;
}
