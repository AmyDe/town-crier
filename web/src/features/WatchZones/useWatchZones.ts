import { useState, useEffect, useCallback } from 'react';
import type { WatchZoneSummary } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';

interface WatchZonesState {
  zones: readonly WatchZoneSummary[];
  isLoading: boolean;
  error: string | null;
}

export function useWatchZones(repository: WatchZoneRepository) {
  const [state, setState] = useState<WatchZonesState>({
    zones: [],
    isLoading: true,
    error: null,
  });

  const loadZones = useCallback(async () => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const zones = await repository.list();
      setState({ zones, isLoading: false, error: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({ ...prev, isLoading: false, error: message }));
    }
  }, [repository]);

  useEffect(() => {
    loadZones();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const deleteZone = useCallback(async (zoneId: string) => {
    try {
      await repository.delete(zoneId);
      await loadZones();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({ ...prev, error: message }));
    }
  }, [repository, loadZones]);

  return {
    zones: state.zones,
    isLoading: state.isLoading,
    error: state.error,
    deleteZone,
  };
}
