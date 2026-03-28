import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApiClient } from '../../api/useApiClient';
import { ApiGroupsRepository } from './ApiGroupsRepository';
import { GroupsListPage } from './GroupsListPage';

export function ConnectedGroupsListPage() {
  const client = useApiClient();
  const navigate = useNavigate();
  const repository = useMemo(() => new ApiGroupsRepository(client), [client]);

  return (
    <GroupsListPage
      repository={repository}
      onCreateClick={() => navigate('/groups/new')}
    />
  );
}
