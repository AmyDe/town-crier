import { useState, useCallback } from 'react';
import type { SettingsRepository } from '../../domain/ports/settings-repository';
import type { UserProfile } from '../../domain/types';
import { useFetchData } from '../../hooks/useFetchData';

export function useUserProfile(
  repository: SettingsRepository,
  logout: () => void,
) {
  const [isExporting, setIsExporting] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const { data: profile, isLoading, error: fetchError } = useFetchData<UserProfile>(
    () => repository.fetchProfile(),
    [repository],
  );

  const error = actionError ?? fetchError;

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
      const message = err instanceof Error ? err.message : 'Failed to export data';
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
      const message = err instanceof Error ? err.message : 'Failed to delete account';
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
  };
}
