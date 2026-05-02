import { useState, useCallback } from 'react';
import type { ApplicationUid, PlanningApplication } from '../../domain/types';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
import { useFetchData } from '../../hooks/useFetchData';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

export function useSavedApplication(
  repository: SavedApplicationRepository,
  uid: ApplicationUid,
) {
  const { data: savedList, isLoading, error: fetchError } = useFetchData(
    () => repository.listSaved(),
    [repository, uid],
  );

  const isSavedFromFetch = savedList?.some((s) => s.applicationUid === uid) ?? false;

  const [optimistic, setOptimistic] = useState<{
    isSaved: boolean;
    error: string | null;
  } | null>(null);

  const toggleSave = useCallback(
    async (application: PlanningApplication) => {
      const wasSaved = optimistic?.isSaved ?? isSavedFromFetch;
      setOptimistic({ isSaved: !wasSaved, error: null });

      try {
        if (wasSaved) {
          await repository.remove(uid);
        } else {
          await repository.save(application);
        }
      } catch (err: unknown) {
        const message = extractErrorMessage(err, 'Unknown error');
        setOptimistic({ isSaved: wasSaved, error: message });
      }
    },
    [repository, uid, optimistic, isSavedFromFetch],
  );

  return {
    isSaved: optimistic?.isSaved ?? isSavedFromFetch,
    isLoading,
    error: optimistic?.error ?? fetchError,
    toggleSave,
  };
}
