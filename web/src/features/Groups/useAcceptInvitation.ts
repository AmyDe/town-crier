import type { InvitationId } from '../../domain/types';
import type { GroupsRepository } from '../../domain/ports/groups-repository';
import { useFetchData } from '../../hooks/useFetchData';

export function useAcceptInvitation(repository: GroupsRepository, invitationId: InvitationId) {
  const { data: isAccepted, isLoading, error } = useFetchData(
    async () => {
      await repository.acceptInvitation(invitationId);
      return true;
    },
    [repository, invitationId],
  );

  return { isLoading, isAccepted: isAccepted ?? false, error };
}
