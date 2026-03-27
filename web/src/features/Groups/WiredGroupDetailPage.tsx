import { useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { useAuth0 } from '@auth0/auth0-react';
import { useApiClient } from '../../api/useApiClient';
import { asGroupId } from '../../domain/types';
import { ApiGroupsRepository } from './ApiGroupsRepository';
import { GroupDetailPage } from './GroupDetailPage';

export function WiredGroupDetailPage() {
  const client = useApiClient();
  const { groupId } = useParams<{ groupId: string }>();
  const { user } = useAuth0();

  const repository = useMemo(() => new ApiGroupsRepository(client), [client]);

  if (!groupId) {
    return <div>Group not found</div>;
  }

  return (
    <GroupDetailPage
      repository={repository}
      groupId={asGroupId(groupId)}
      currentUserId={user?.sub ?? ''}
    />
  );
}
