import { useState, useEffect, useCallback } from 'react';
import type { ApplicationUid } from '../../domain/types';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';

interface SavedApplicationState {
  isSaved: boolean;
  isLoading: boolean;
  error: string | null;
}

export function useSavedApplication(
  repository: SavedApplicationRepository,
  uid: ApplicationUid,
) {
  const [state, setState] = useState<SavedApplicationState>({
    isSaved: false,
    isLoading: true,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;

    async function checkSavedStatus() {
      try {
        const savedList = await repository.listSaved();
        if (!cancelled) {
          const found = savedList.some((s) => s.applicationUid === uid);
          setState({ isSaved: found, isLoading: false, error: null });
        }
      } catch (err: unknown) {
        if (!cancelled) {
          const message = err instanceof Error ? err.message : 'Unknown error';
          setState({ isSaved: false, isLoading: false, error: message });
        }
      }
    }

    checkSavedStatus();
    return () => {
      cancelled = true;
    };
  }, [repository, uid]);

  const toggleSave = useCallback(async () => {
    const wasSaved = state.isSaved;
    setState((prev) => ({ ...prev, isSaved: !wasSaved, error: null }));

    try {
      if (wasSaved) {
        await repository.remove(uid);
      } else {
        await repository.save(uid);
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setState((prev) => ({ ...prev, isSaved: wasSaved, error: message }));
    }
  }, [repository, uid, state.isSaved]);

  return { ...state, toggleSave };
}
