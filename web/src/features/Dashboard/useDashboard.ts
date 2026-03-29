import type { DashboardPort } from '../../domain/ports/dashboard-port';
import type { PlanningApplicationSummary, WatchZoneSummary } from '../../domain/types';
import { useFetchData } from '../../hooks/useFetchData';

interface DashboardData {
  zones: readonly WatchZoneSummary[];
  recentApplications: readonly PlanningApplicationSummary[];
}

export function useDashboard(port: DashboardPort) {
  const { data, isLoading, error, refresh } = useFetchData<DashboardData>(
    async () => {
      const zones = await port.fetchWatchZones();

      const uniqueAuthorityIds = [...new Set(zones.map(z => z.authorityId))];
      const applicationsByAuthority = await Promise.all(
        uniqueAuthorityIds.map(id => port.fetchRecentApplications(id)),
      );
      const recentApplications = applicationsByAuthority.flat();

      return { zones, recentApplications };
    },
    [port],
  );

  return {
    zones: data?.zones ?? [],
    recentApplications: data?.recentApplications ?? [],
    isLoading,
    error,
    refresh,
  };
}
