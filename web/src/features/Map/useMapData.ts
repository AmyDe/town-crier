import { useState, useEffect, useCallback } from 'react';
import type { PlanningApplication, WatchZoneSummary } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';

interface MapDataState {
  readonly zones: readonly WatchZoneSummary[];
  readonly applications: readonly PlanningApplication[];
  readonly isLoading: boolean;
  readonly error: string | null;
}

export function useMapData(port: MapPort) {
  const [state, setState] = useState<MapDataState>({
    zones: [],
    applications: [],
    isLoading: true,
    error: null,
  });

  const loadData = useCallback(async () => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const zones = await port.fetchWatchZones();

      const uniqueAuthorityIds = [...new Set(zones.map(z => z.authorityId))];

      const applicationArrays = await Promise.all(
        uniqueAuthorityIds.map(id => port.fetchApplicationsByAuthority(id)),
      );

      const applications = applicationArrays.flat();

      setState({
        zones,
        applications,
        isLoading: false,
        error: null,
      });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: message,
      }));
    }
  }, [port]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const zones = await port.fetchWatchZones();
        const uniqueAuthorityIds = [...new Set(zones.map(z => z.authorityId))];
        const applicationArrays = await Promise.all(
          uniqueAuthorityIds.map(id => port.fetchApplicationsByAuthority(id)),
        );
        const applications = applicationArrays.flat();
        if (!cancelled) {
          setState({ zones, applications, isLoading: false, error: null });
        }
      } catch (err: unknown) {
        if (!cancelled) {
          const message = err instanceof Error ? err.message : 'An error occurred';
          setState(prev => ({ ...prev, isLoading: false, error: message }));
        }
      }
    })();
    return () => { cancelled = true; };
  }, [port]);

  return { ...state, refresh: loadData };
}
