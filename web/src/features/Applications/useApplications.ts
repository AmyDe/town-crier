import { useState, useCallback } from 'react';
import type { WatchZoneSummary, PlanningApplicationSummary } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import { useFetchData } from '../../hooks/useFetchData';

export function useApplications(port: ApplicationsBrowsePort) {
  const [selectedZone, setSelectedZone] = useState<WatchZoneSummary | null>(null);
  const [fetchKey, setFetchKey] = useState(0);

  const { data, isLoading, error } = useFetchData<readonly PlanningApplicationSummary[]>(
    () => port.fetchByZone(selectedZone!.id),
    [selectedZone?.id, fetchKey],
    { enabled: selectedZone !== null },
  );

  const selectZone = useCallback(
    (zone: WatchZoneSummary | null) => {
      setSelectedZone(zone);
      setFetchKey((k) => k + 1);
    },
    [],
  );

  return {
    selectedZone,
    applications: data ?? [],
    isLoading,
    error,
    selectZone,
  };
}
