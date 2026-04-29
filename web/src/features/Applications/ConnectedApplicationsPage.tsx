import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { applicationsApi } from '../../api/applications';
import { watchZonesApi } from '../../api/watchZones';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import { ApplicationsPage } from './ApplicationsPage';

export function ConnectedApplicationsPage() {
  const client = useApiClient();

  const browsePort: ApplicationsBrowsePort = useMemo(
    () => ({
      fetchByZone: (zoneId) =>
        applicationsApi(client).getByZone(zoneId as string),
    }),
    [client],
  );

  const zonesPort = useMemo(
    () => ({
      fetchZones: () => watchZonesApi(client).list(),
    }),
    [client],
  );

  return (
    <ApplicationsPage
      zonesPort={zonesPort}
      browsePort={browsePort}
    />
  );
}
