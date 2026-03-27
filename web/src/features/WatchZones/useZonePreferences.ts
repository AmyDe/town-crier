import { useState, useEffect, useCallback } from 'react';
import type { ZoneNotificationPreferences, UpdateZonePreferencesRequest } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';

interface ZonePreferencesState {
  preferences: ZoneNotificationPreferences | null;
  isLoading: boolean;
  isSaving: boolean;
  error: string | null;
}

export function useZonePreferences(repository: WatchZoneRepository, zoneId: string) {
  const [state, setState] = useState<ZonePreferencesState>({
    preferences: null,
    isLoading: true,
    isSaving: false,
    error: null,
  });

  const loadPreferences = useCallback(async () => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const preferences = await repository.getPreferences(zoneId);
      setState({ preferences, isLoading: false, isSaving: false, error: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({ ...prev, isLoading: false, error: message }));
    }
  }, [repository, zoneId]);

  useEffect(() => {
    let cancelled = false;
    repository.getPreferences(zoneId).then(preferences => {
      if (!cancelled) {
        setState({ preferences, isLoading: false, isSaving: false, error: null });
      }
    }).catch((err: unknown) => {
      if (!cancelled) {
        const message = err instanceof Error ? err.message : 'An error occurred';
        setState(prev => ({ ...prev, isLoading: false, error: message }));
      }
    });
    return () => { cancelled = true; };
  }, [repository, zoneId]);

  const updatePreferences = useCallback(async (data: UpdateZonePreferencesRequest) => {
    setState(prev => ({ ...prev, isSaving: true, error: null }));
    try {
      await repository.updatePreferences(zoneId, data);
      await loadPreferences();
      setState(prev => ({ ...prev, isSaving: false }));
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({ ...prev, isSaving: false, error: message }));
    }
  }, [repository, zoneId, loadPreferences]);

  return {
    preferences: state.preferences,
    isLoading: state.isLoading,
    isSaving: state.isSaving,
    error: state.error,
    updatePreferences,
  };
}
