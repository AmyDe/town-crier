import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { applicationsApi } from '../../api/applications';
import { watchZonesApi } from '../../api/watchZones';
import { savedApplicationsApi } from '../../api/savedApplications';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
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

  const savedRepository: SavedApplicationRepository = useMemo(
    () => ({
      listSaved: () => savedApplicationsApi(client).list(),
      save: (uid) => savedApplicationsApi(client).save(uid),
      remove: (uid) => savedApplicationsApi(client).remove(uid),
    }),
    [client],
  );

  return (
    <ApplicationsPage
      zonesPort={zonesPort}
      browsePort={browsePort}
      savedRepository={savedRepository}
    />
  );
}
