import { useState, useCallback } from 'react';
import type { SettingsRepository } from '../../domain/ports/settings-repository';
import type { UpdateProfileRequest, UserProfile } from '../../domain/types';
import { useFetchData } from '../../hooks/useFetchData';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

export function useUserProfile(
  repository: SettingsRepository,
  logout: () => void,
) {
  const [isExporting, setIsExporting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [optimisticProfile, setOptimisticProfile] = useState<UserProfile | null>(null);

  const { data: fetchedProfile, isLoading, error: fetchError } = useFetchData<UserProfile>(
    () => repository.fetchProfile(),
    [repository],
  );

  const profile = optimisticProfile ?? fetchedProfile;
  const error = actionError ?? fetchError;

  const updatePreferences = useCallback(async (changes: Partial<UpdateProfileRequest>) => {
    const base = optimisticProfile ?? fetchedProfile;
    if (!base) return;

    const request: UpdateProfileRequest = {
      pushEnabled: base.pushEnabled,
      emailDigestEnabled: base.emailDigestEnabled,
      emailInstantEnabled: base.emailInstantEnabled,
      digestDay: base.digestDay,
      ...changes,
    };

    const previous = optimisticProfile;
    setOptimisticProfile({ ...base, ...changes });
    setActionError(null);

    try {
      const updated = await repository.updateProfile(request);
      setOptimisticProfile(updated);
    } catch (err) {
      setOptimisticProfile(previous);
      const message = extractErrorMessage(err, 'Failed to update preferences');
      setActionError(message);
    }
  }, [optimisticProfile, fetchedProfile, repository]);

  const exportData = useCallback(async () => {
    setIsExporting(true);
    setActionError(null);
    try {
      const blob = await repository.exportData();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'town-crier-data.json';
      link.click();
      URL.revokeObjectURL(url);
      setIsExporting(false);
    } catch (err) {
      const message = extractErrorMessage(err, 'Failed to export data');
      setIsExporting(false);
      setActionError(message);
    }
  }, [repository]);

  const deleteAccount = useCallback(async () => {
    setIsDeleting(true);
    setActionError(null);
    try {
      await repository.deleteAccount();
      setIsDeleting(false);
      logout();
    } catch (err) {
      const message = extractErrorMessage(err, 'Failed to delete account');
      setIsDeleting(false);
      setActionError(message);
    }
  }, [repository, logout]);

  return {
    profile,
    isLoading,
    isExporting,
    isDeleting,
    error,
    exportData,
    deleteAccount,
    updatePreferences,
  };
}
