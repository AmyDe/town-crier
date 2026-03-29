import { useState, useCallback } from 'react';
import type { ZoneNotificationPreferences, UpdateZonePreferencesRequest } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import { useFetchData } from '../../hooks/useFetchData';

export function useZonePreferences(repository: WatchZoneRepository, zoneId: string) {
  const [isSaving, setIsSaving] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const { data: preferences, isLoading, error: fetchError, refresh } = useFetchData<ZoneNotificationPreferences>(
    () => repository.getPreferences(zoneId),
    [repository, zoneId],
  );

  const updatePreferences = useCallback(async (data: UpdateZonePreferencesRequest) => {
    setIsSaving(true);
    setActionError(null);
    try {
      await repository.updatePreferences(zoneId, data);
      refresh();
      setIsSaving(false);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setIsSaving(false);
      setActionError(message);
    }
  }, [repository, zoneId, refresh]);

  return {
    preferences,
    isLoading,
    isSaving,
    error: actionError ?? fetchError,
    updatePreferences,
  };
}
