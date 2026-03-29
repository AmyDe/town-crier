import { useState, useCallback } from 'react';
import type { GroupId } from '../../domain/types';
import type { GroupsRepository } from '../../domain/ports/groups-repository';
import { useFetchData } from '../../hooks/useFetchData';

export function useGroupDetail(repository: GroupsRepository, groupId: GroupId) {
  const [actionError, setActionError] = useState<string | null>(null);

  const { data: group, isLoading, error: fetchError, refresh } = useFetchData(
    () => repository.getGroup(groupId),
    [repository, groupId],
  );

  const inviteMember = useCallback(
    async (email: string) => {
      setActionError(null);
      try {
        await repository.inviteMember(groupId, { inviteeEmail: email });
        refresh();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to send invitation';
        setActionError(message);
      }
    },
    [repository, groupId, refresh],
  );

  const removeMember = useCallback(
    async (memberUserId: string) => {
      setActionError(null);
      try {
        await repository.removeMember(groupId, memberUserId);
        refresh();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to remove member';
        setActionError(message);
      }
    },
    [repository, groupId, refresh],
  );

  const deleteGroup = useCallback(async () => {
    setActionError(null);
    try {
      await repository.deleteGroup(groupId);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to delete group';
      setActionError(message);
    }
  }, [repository, groupId]);

  return {
    group,
    isLoading,
    error: fetchError,
    actionError,
    inviteMember,
    removeMember,
    deleteGroup,
    refresh,
  };
}
