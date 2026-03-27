import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { watchZonesApi } from '../../api/watchZones';
import { applicationsApi } from '../../api/applications';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import type { MapApplicationsPort } from '../../domain/ports/map-applications-port';
import { MapPage } from './MapPage';

export function ConnectedMapPage() {
  const client = useApiClient();

  const watchZoneRepo: WatchZoneRepository = useMemo(
    () => ({
      list: () => watchZonesApi(client).list(),
      create: (data) => watchZonesApi(client).create(data),
      delete: (zoneId) => watchZonesApi(client).delete(zoneId),
      getPreferences: (zoneId) => watchZonesApi(client).getPreferences(zoneId),
      updatePreferences: (zoneId, data) =>
        watchZonesApi(client).updatePreferences(zoneId, data),
    }),
    [client],
  );

  const applicationsPort: MapApplicationsPort = useMemo(
    () => ({
      fetchByAuthority: (authorityId) =>
        applicationsApi(client).getByAuthority(authorityId),
    }),
    [client],
  );

  return (
    <MapPage watchZoneRepo={watchZoneRepo} applicationsPort={applicationsPort} />
  );
}
