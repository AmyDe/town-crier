import { useState, useEffect, useCallback } from 'react';
import type { PlanningApplication } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import type { MapApplicationsPort } from '../../domain/ports/map-applications-port';

interface MapCenter {
  readonly lat: number;
  readonly lng: number;
}

const DEFAULT_CENTER: MapCenter = { lat: 51.505, lng: -0.09 };

interface MapDataState {
  applications: readonly PlanningApplication[];
  center: MapCenter;
  isLoading: boolean;
  error: string | null;
}

export function useMapData(
  watchZoneRepo: WatchZoneRepository,
  applicationsPort: MapApplicationsPort,
) {
  const [state, setState] = useState<MapDataState>({
    applications: [],
    center: DEFAULT_CENTER,
    isLoading: true,
    error: null,
  });

  const loadData = useCallback(async () => {
    setState((prev) => ({ ...prev, isLoading: true, error: null }));

    try {
      const zones = await watchZoneRepo.list();

      const center: MapCenter =
        zones.length > 0
          ? { lat: zones[0]!.latitude, lng: zones[0]!.longitude }
          : DEFAULT_CENTER;

      if (zones.length === 0) {
        setState({ applications: [], center, isLoading: false, error: null });
        return;
      }

      // Deduplicate authority IDs across zones
      const uniqueAuthorityIds = [...new Set(zones.map((z) => z.authorityId))];

      const applicationArrays = await Promise.all(
        uniqueAuthorityIds.map((authorityId) =>
          applicationsPort.fetchByAuthority(authorityId),
        ),
      );

      const allApplications = applicationArrays.flat();

      // Filter out applications without coordinates
      const withCoordinates = allApplications.filter(
        (app): app is PlanningApplication =>
          app.latitude !== null && app.longitude !== null,
      );

      setState({
        applications: withCoordinates,
        center,
        isLoading: false,
        error: null,
      });
    } catch (err: unknown) {
      setState((prev) => ({
        ...prev,
        isLoading: false,
        error: err instanceof Error ? err.message : String(err),
      }));
    }
  }, [watchZoneRepo, applicationsPort]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  return {
    applications: state.applications,
    center: state.center,
    isLoading: state.isLoading,
    error: state.error,
  };
}
