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
        applicationsApi(client).getByZonePaged(query.zoneId, {
          sort: query.sort,
          status: query.status,
          unread: query.unread,
          cursor: query.cursor,
        }),
      // Whole-zone unread total: exhausts the unread-only paged query (following
      // X-Next-Cursor); `sort` is irrelevant to a count.
      countUnread: async (zoneId) => {
        let total = 0;
        let cursor: string | null = null;
        do {
          const page = await applicationsApi(client).getByZonePaged(zoneId, {
            sort: 'newest',
            status: null,
            unread: true,
            cursor,
          });
          total += page.rows.length;
          cursor = page.nextCursor;
        } while (cursor !== null);
        return total;
      },
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
