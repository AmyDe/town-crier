import { useMemo } from 'react';
import { useNavigate } from 'react-router';
import { useApiClient } from '../../api/useApiClient';
import { geocodingApi } from '../../api/geocoding';
import { useProfileRepository } from '../../auth/profile-context';
import { useFetchData } from '../../hooks/useFetchData';
import { ApiWatchZoneRepository } from './ApiWatchZoneRepository';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import { WatchZoneCreatePage } from './WatchZoneCreatePage';

export function ConnectedWatchZoneCreatePage() {
  const client = useApiClient();
  const navigate = useNavigate();
  const repository = useMemo(() => new ApiWatchZoneRepository(client), [client]);
  const geocodingPort: GeocodingPort = useMemo(
    () => ({ geocode: (postcode) => geocodingApi(client).geocode(postcode) }),
    [client],
  );

  const profileRepository = useProfileRepository();
  const { data: profile } = useFetchData(
    () => profileRepository.fetchProfile(),
    [profileRepository],
  );

  return (
    <WatchZoneCreatePage
      repository={repository}
      geocodingPort={geocodingPort}
      navigate={navigate}
      tier={profile?.tier ?? 'Free'}
    />
  );
}
