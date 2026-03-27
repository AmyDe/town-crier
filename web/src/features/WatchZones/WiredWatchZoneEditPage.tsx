import { useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { useApiClient } from '../../api/useApiClient';
import { ApiWatchZoneRepository } from './ApiWatchZoneRepository';
import { useWatchZones } from './useWatchZones';
import { WatchZoneEditPage } from './WatchZoneEditPage';

export function WiredWatchZoneEditPage() {
  const { zoneId } = useParams<{ zoneId: string }>();
  const client = useApiClient();
  const repository = useMemo(() => new ApiWatchZoneRepository(client), [client]);
  const { zones, isLoading } = useWatchZones(repository);

  const zone = zones.find((z) => z.id === zoneId);

  if (isLoading) {
    return <p>Loading...</p>;
  }

  if (!zone) {
    return <p>Watch zone not found.</p>;
  }

  return <WatchZoneEditPage repository={repository} zone={zone} />;
}
