import { useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApiClient } from '../../api/useApiClient';
import { geocodingApi } from '../../api/geocoding';
import { ApiWatchZoneRepository } from './ApiWatchZoneRepository';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import { WatchZoneCreatePage } from './WatchZoneCreatePage';

export function WiredWatchZoneCreatePage() {
  const client = useApiClient();
  const navigate = useNavigate();
  const repository = useMemo(() => new ApiWatchZoneRepository(client), [client]);
  const geocodingPort: GeocodingPort = useMemo(
    () => ({ geocode: (postcode) => geocodingApi(client).geocode(postcode) }),
    [client],
  );

  return (
    <WatchZoneCreatePage
      repository={repository}
      geocodingPort={geocodingPort}
      navigate={navigate}
    />
  );
}
