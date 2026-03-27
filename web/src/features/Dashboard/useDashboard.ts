import { useState, useEffect, useCallback } from 'react';
import type { DashboardPort } from '../../domain/ports/dashboard-port';
import type { PlanningApplicationSummary, WatchZoneSummary } from '../../domain/types';

interface DashboardState {
  zones: readonly WatchZoneSummary[];
  recentApplications: readonly PlanningApplicationSummary[];
  isLoading: boolean;
  error: string | null;
}

export function useDashboard(port: DashboardPort) {
  const [state, setState] = useState<DashboardState>({
    zones: [],
    recentApplications: [],
    isLoading: true,
    error: null,
  });

  const load = useCallback(async () => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const zones = await port.fetchWatchZones();

      const uniqueAuthorityIds = [...new Set(zones.map(z => z.authorityId))];
      const applicationsByAuthority = await Promise.all(
        uniqueAuthorityIds.map(id => port.fetchRecentApplications(id)),
      );
      const recentApplications = applicationsByAuthority.flat();

      setState({ zones, recentApplications, isLoading: false, error: null });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load dashboard';
      setState(prev => ({ ...prev, isLoading: false, error: message }));
    }
  }, [port]);

  useEffect(() => {
    load();
  }, [load]);

  return { ...state, refresh: load };
}
