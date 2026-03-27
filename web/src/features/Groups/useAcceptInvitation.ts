import { useState, useEffect } from 'react';
import type { InvitationId } from '../../domain/types';
import type { GroupsRepository } from '../../domain/ports/groups-repository';

interface AcceptInvitationState {
  isLoading: boolean;
  isAccepted: boolean;
  error: string | null;
}

export function useAcceptInvitation(repository: GroupsRepository, invitationId: InvitationId) {
  const [state, setState] = useState<AcceptInvitationState>({
    isLoading: true,
    isAccepted: false,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;

    async function accept() {
      try {
        await repository.acceptInvitation(invitationId);
        if (!cancelled) {
          setState({ isLoading: false, isAccepted: true, error: null });
        }
      } catch (err) {
        if (!cancelled) {
          const message = err instanceof Error ? err.message : 'Failed to accept invitation';
          setState({ isLoading: false, isAccepted: false, error: message });
        }
      }
    }

    void accept();

    return () => {
      cancelled = true;
    };
  }, [repository, invitationId]);

  return state;
}
