import { useCallback, useMemo } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { useApiClient } from '../../api/useApiClient';
import { ApiSettingsRepository } from './ApiSettingsRepository';
import { SettingsPage } from './SettingsPage';
import { createRedeemOfferCodeClient } from '../offerCode/api/redeemOfferCode';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

export function ConnectedSettingsPage() {
  const client = useApiClient();
  const { getAccessTokenSilently } = useAuth0();

  const repository = useMemo(
    () => new ApiSettingsRepository(client, API_BASE_URL, getAccessTokenSilently),
    [client, getAccessTokenSilently],
  );

  const redeemOfferCodeClient = useMemo(
    () => createRedeemOfferCodeClient(
      () => getAccessTokenSilently(),
      API_BASE_URL,
    ),
    [getAccessTokenSilently],
  );

  // Drop Auth0's cached token so the next request carries the new subscription_tier claim.
  const handleRedeemSuccess = useCallback(async () => {
    try {
      await getAccessTokenSilently({ cacheMode: 'off' });
    } catch {
      // Best effort: tier-gated UI only lags until the next natural token refresh.
    }
  }, [getAccessTokenSilently]);

  return (
    <SettingsPage
      repository={repository}
      redeemOfferCodeClient={redeemOfferCodeClient}
      onRedeemSuccess={handleRedeemSuccess}
    />
  );
}
