import type { PlanningApplication, WatchZoneSummary } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { useFetchData } from '../../hooks/useFetchData';

interface MapData {
  zones: readonly WatchZoneSummary[];
  applications: readonly PlanningApplication[];
}

export function useMapData(port: MapPort) {
  const { data, isLoading, error, refresh } = useFetchData<MapData>(
    async () => {
      const zones = await port.fetchWatchZones();

      const uniqueAuthorityIds = [...new Set(zones.map(z => z.authorityId))];
      const applicationArrays = await Promise.all(
        uniqueAuthorityIds.map(id => port.fetchApplicationsByAuthority(id)),
      );
      const applications = applicationArrays.flat();

      return { zones, applications };
    },
    [port],
  );

  return {
    zones: data?.zones ?? [],
    applications: data?.applications ?? [],
    isLoading,
    error,
    refresh,
  };
}
