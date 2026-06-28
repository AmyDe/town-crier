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
      // Whole-zone unread total for the "Unread (N)" chip: exhaust the
      // unread-only paged query (following X-Next-Cursor) and sum the rows.
      // Unread sets are small, so this is typically a single request; `sort` is
      // irrelevant to a count. Independent of the main list's pagination.
      countUnread: async (zoneId) => {
        let total = 0;
        let cursor: string | null = null;
        do {
          const page = await applicationsApi(client).getByZonePaged(zoneId as string, {
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
