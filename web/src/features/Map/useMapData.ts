import type { ApplicationUid, PlanningApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { useFetchData } from '../../hooks/useFetchData';

interface MapData {
  readonly applications: readonly PlanningApplication[];
  readonly fetchedSavedUids: ReadonlySet<ApplicationUid>;
}

export function useMapData(port: MapPort) {
  const { data, isLoading, error, refresh } = useFetchData<MapData>(
    async () => {
      const [authorities, savedApps] = await Promise.all([
        port.fetchMyAuthorities(),
        port.fetchSavedApplications(),
      ]);

      const uniqueAuthorityIds = [...new Set(authorities.map(a => a.id))];
      const applicationArrays = await Promise.all(
        uniqueAuthorityIds.map(id => port.fetchApplicationsByAuthority(id)),
      );

      return {
        applications: applicationArrays.flat(),
        fetchedSavedUids: new Set(savedApps.map(s => s.applicationUid)),
      };
    },
    [port],
  );

  const savedUids: ReadonlySet<ApplicationUid> = data?.fetchedSavedUids ?? new Set();

  return {
    applications: data?.applications ?? [],
    savedUids,
    isLoading,
    error,
    refresh,
  };
}
