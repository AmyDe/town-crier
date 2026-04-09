import { useState, useCallback } from 'react';
import type { WatchZoneSummary } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import { useFetchData } from '../../hooks/useFetchData';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

export function useWatchZones(repository: WatchZoneRepository) {
  const [actionError, setActionError] = useState<string | null>(null);

  const { data: zones, isLoading, error: fetchError, refresh } = useFetchData<readonly WatchZoneSummary[]>(
    () => repository.list(),
    [repository],
  );

  const deleteZone = useCallback(async (zoneId: string) => {
    try {
      await repository.delete(zoneId);
      refresh();
    } catch (err: unknown) {
      const message = extractErrorMessage(err);
      setActionError(message);
    }
  }, [repository, refresh]);

  return {
    zones: zones ?? [],
    isLoading,
    error: actionError ?? fetchError,
    deleteZone,
  };
}
