import { useMemo } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { useApiClient } from '../../api/useApiClient';
import { ApiSettingsRepository } from './ApiSettingsRepository';
import { SettingsPage } from './SettingsPage';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

export function ConnectedSettingsPage() {
  const client = useApiClient();
  const { getAccessTokenSilently } = useAuth0();
  const repository = useMemo(
    () => new ApiSettingsRepository(client, API_BASE_URL, getAccessTokenSilently),
    [client, getAccessTokenSilently],
  );

  return <SettingsPage repository={repository} />;
}
