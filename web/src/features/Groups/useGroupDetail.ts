import { useState, useEffect, useCallback } from 'react';
import type { GroupDetail, GroupId } from '../../domain/types';
import type { GroupsRepository } from '../../domain/ports/groups-repository';

interface GroupDetailState {
  group: GroupDetail | null;
  isLoading: boolean;
  error: string | null;
  actionError: string | null;
}

export function useGroupDetail(repository: GroupsRepository, groupId: GroupId) {
  const [state, setState] = useState<GroupDetailState>({
    group: null,
    isLoading: true,
    error: null,
    actionError: null,
  });

  const loadGroup = useCallback(async () => {
    setState((prev) => ({ ...prev, isLoading: true, error: null }));
    try {
      const group = await repository.getGroup(groupId);
      setState({ group, isLoading: false, error: null, actionError: null });
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to load group';
      setState({ group: null, isLoading: false, error: message, actionError: null });
    }
  }, [repository, groupId]);

  useEffect(() => {
    let cancelled = false;
    repository.getGroup(groupId).then(group => {
      if (!cancelled) {
        setState({ group, isLoading: false, error: null, actionError: null });
      }
    }).catch((err: unknown) => {
      if (!cancelled) {
        const message = err instanceof Error ? err.message : 'Failed to load group';
        setState({ group: null, isLoading: false, error: message, actionError: null });
      }
    });
    return () => { cancelled = true; };
  }, [repository, groupId]);

  const inviteMember = useCallback(
    async (email: string) => {
      setState((prev) => ({ ...prev, actionError: null }));
      try {
        await repository.inviteMember(groupId, { inviteeEmail: email });
        await loadGroup();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to send invitation';
        setState((prev) => ({ ...prev, actionError: message }));
      }
    },
    [repository, groupId, loadGroup],
  );

  const removeMember = useCallback(
    async (memberUserId: string) => {
      setState((prev) => ({ ...prev, actionError: null }));
      try {
        await repository.removeMember(groupId, memberUserId);
        await loadGroup();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to remove member';
        setState((prev) => ({ ...prev, actionError: message }));
      }
    },
    [repository, groupId, loadGroup],
  );

  const deleteGroup = useCallback(async () => {
    setState((prev) => ({ ...prev, actionError: null }));
    try {
      await repository.deleteGroup(groupId);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete group';
      setState((prev) => ({ ...prev, actionError: message }));
    }
  }, [repository, groupId]);

  return {
    ...state,
    inviteMember,
    removeMember,
    deleteGroup,
    refresh: loadGroup,
  };
}
