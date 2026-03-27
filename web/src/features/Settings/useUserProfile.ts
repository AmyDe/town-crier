import { useState, useEffect, useCallback } from 'react';
import type { SettingsRepository } from '../../domain/ports/settings-repository';
import type { UserProfile } from '../../domain/types';

interface UserProfileState {
  profile: UserProfile | null;
  isLoading: boolean;
  isExporting: boolean;
  isDeleting: boolean;
  error: string | null;
}

export function useUserProfile(
  repository: SettingsRepository,
  logout: () => void,
) {
  const [state, setState] = useState<UserProfileState>({
    profile: null,
    isLoading: true,
    isExporting: false,
    isDeleting: false,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;

    async function load() {
      try {
        const profile = await repository.fetchProfile();
        if (!cancelled) {
          setState(prev => ({ ...prev, profile, isLoading: false }));
        }
      } catch (err) {
        if (!cancelled) {
          const message = err instanceof Error ? err.message : 'Failed to load profile';
          setState(prev => ({ ...prev, isLoading: false, error: message }));
        }
      }
    }

    load();
    return () => { cancelled = true; };
  }, [repository]);

  const exportData = useCallback(async () => {
    setState(prev => ({ ...prev, isExporting: true, error: null }));
    try {
      const blob = await repository.exportData();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = 'town-crier-data.json';
      link.click();
      URL.revokeObjectURL(url);
      setState(prev => ({ ...prev, isExporting: false }));
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to export data';
      setState(prev => ({ ...prev, isExporting: false, error: message }));
    }
  }, [repository]);

  const deleteAccount = useCallback(async () => {
    setState(prev => ({ ...prev, isDeleting: true, error: null }));
    try {
      await repository.deleteAccount();
      setState(prev => ({ ...prev, isDeleting: false }));
      logout();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete account';
      setState(prev => ({ ...prev, isDeleting: false, error: message }));
    }
  }, [repository, logout]);

  return {
    ...state,
    exportData,
    deleteAccount,
  };
}
