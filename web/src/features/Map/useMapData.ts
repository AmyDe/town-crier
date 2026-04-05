import type { PlanningApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { useFetchData } from '../../hooks/useFetchData';

export function useMapData(port: MapPort) {
  const { data, isLoading, error, refresh } = useFetchData<readonly PlanningApplication[]>(
    async () => {
      const authorities = await port.fetchMyAuthorities();

      const uniqueAuthorityIds = [...new Set(authorities.map(a => a.id))];
      const applicationArrays = await Promise.all(
        uniqueAuthorityIds.map(id => port.fetchApplicationsByAuthority(id)),
      );
      return applicationArrays.flat();
    },
    [port],
  );

  return {
    applications: data ?? [],
    isLoading,
    error,
    refresh,
  };
}
