import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { applicationsApi } from '../../api/applications';
import { watchZonesApi } from '../../api/watchZones';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import { ApiNotificationStateRepository } from './ApiNotificationStateRepository';
import { ApplicationsPage } from './ApplicationsPage';

export function ConnectedApplicationsPage() {
  const client = useApiClient();

  const browsePort: ApplicationsBrowsePort = useMemo(
    () => ({
      fetchByZone: (query) =>
        applicationsApi(client).getByZonePaged(query.zoneId as string, {
          sort: query.sort,
          status: query.status,
          unread: query.unread,
          cursor: query.cursor,
        }),
    }),
    [client],
  );

  const zonesPort = useMemo(
    () => ({
      fetchZones: () => watchZonesApi(client).list(),
    }),
    [client],
  );

  const notificationStateRepository = useMemo(
    () => new ApiNotificationStateRepository(client),
    [client],
  );

  return (
    <ApplicationsPage
      zonesPort={zonesPort}
      browsePort={browsePort}
      notificationStateRepository={notificationStateRepository}
    />
  );
}
