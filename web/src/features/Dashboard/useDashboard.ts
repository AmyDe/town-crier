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

      const applicationsByZone = await Promise.all(
        zones.map(z => port.fetchRecentApplications(z.id)),
      );
      const recentApplications = applicationsByZone.flat();

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
