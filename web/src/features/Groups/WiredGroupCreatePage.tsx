import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { geocodingApi } from '../../api/geocoding';
import { authoritiesApi } from '../../api/authorities';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import type { AuthoritySearchPort } from '../../domain/ports/authority-search-port';
import { ApiGroupsRepository } from './ApiGroupsRepository';
import { GroupCreatePage } from './GroupCreatePage';

export function WiredGroupCreatePage() {
  const client = useApiClient();

  const repository = useMemo(() => new ApiGroupsRepository(client), [client]);

  const geocodingPort: GeocodingPort = useMemo(
    () => ({
      geocode: (postcode) => geocodingApi(client).geocode(postcode),
    }),
    [client],
  );

  const authoritySearchPort: AuthoritySearchPort = useMemo(
    () => ({
      search: (query) => authoritiesApi(client).list(query),
    }),
    [client],
  );

  return (
    <GroupCreatePage
      repository={repository}
      geocodingPort={geocodingPort}
      authoritySearchPort={authoritySearchPort}
    />
  );
}
