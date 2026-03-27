import { useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { useApiClient } from '../../api/useApiClient';
import { asInvitationId } from '../../domain/types';
import { ApiGroupsRepository } from './ApiGroupsRepository';
import { AcceptInvitationPage } from './AcceptInvitationPage';

export function WiredAcceptInvitationPage() {
  const client = useApiClient();
  const { invitationId } = useParams<{ invitationId: string }>();

  const repository = useMemo(() => new ApiGroupsRepository(client), [client]);

  if (!invitationId) {
    return <div>Invalid invitation</div>;
  }

  return (
    <AcceptInvitationPage
      repository={repository}
      invitationId={asInvitationId(invitationId)}
    />
  );
}
