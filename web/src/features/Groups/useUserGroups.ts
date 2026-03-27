import { useState, useEffect, useCallback } from 'react';
import type { GroupSummary } from '../../domain/types';
import type { GroupsRepository } from '../../domain/ports/groups-repository';

interface UserGroupsState {
  groups: readonly GroupSummary[];
  isLoading: boolean;
  error: string | null;
}

export function useUserGroups(repository: GroupsRepository) {
  const [state, setState] = useState<UserGroupsState>({
    groups: [],
    isLoading: true,
    error: null,
  });

  const loadGroups = useCallback(async () => {
    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    try {
      const groups = await repository.listGroups();
      setState({ groups, isLoading: false, error: null });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load groups';
      setState({ groups: [], isLoading: false, error: message });
    }
  }, [repository]);

  useEffect(() => {
    let cancelled = false;
    repository.listGroups().then(groups => {
      if (!cancelled) {
        setState({ groups, isLoading: false, error: null });
      }
    }).catch((err: unknown) => {
      if (!cancelled) {
        const message = err instanceof Error ? err.message : 'Failed to load groups';
        setState({ groups: [], isLoading: false, error: message });
      }
    });
    return () => { cancelled = true; };
  }, [repository]);

  return { ...state, refresh: loadGroups };
}
