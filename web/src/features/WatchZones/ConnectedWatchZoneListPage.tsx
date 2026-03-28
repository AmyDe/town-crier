import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { ApiWatchZoneRepository } from './ApiWatchZoneRepository';
import { WatchZoneListPage } from './WatchZoneListPage';

export function ConnectedWatchZoneListPage() {
  const client = useApiClient();
  const repository = useMemo(() => new ApiWatchZoneRepository(client), [client]);

  return <WatchZoneListPage repository={repository} />;
}
