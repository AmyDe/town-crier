import { useMemo } from 'react';
import { useParams } from 'react-router-dom';
import { useApiClient } from '../../api/useApiClient';
import { useProfileRepository } from '../../auth/profile-context';
import { useFetchData } from '../../hooks/useFetchData';
import { ApiWatchZoneRepository } from './ApiWatchZoneRepository';
import { useWatchZones } from './useWatchZones';
import { WatchZoneEditPage } from './WatchZoneEditPage';

export function ConnectedWatchZoneEditPage() {
  const { zoneId } = useParams<{ zoneId: string }>();
  const client = useApiClient();
  const repository = useMemo(() => new ApiWatchZoneRepository(client), [client]);
  const { zones, isLoading } = useWatchZones(repository);

  const profileRepository = useProfileRepository();
  const { data: profile } = useFetchData(
    () => profileRepository.fetchProfile(),
    [profileRepository],
  );

  const zone = zones.find((z) => z.id === zoneId);

  if (isLoading) {
    return <p>Loading...</p>;
  }

  if (!zone) {
    return <p>Watch zone not found.</p>;
  }

  return (
    <WatchZoneEditPage
      repository={repository}
      zone={zone}
      tier={profile?.tier ?? 'Free'}
    />
  );
}
