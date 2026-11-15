import { useCallback } from 'react';
import { useAuth0 } from '@auth0/auth0-react';

/**
 * Returns a stable function that resolves with the current Auth0 access token.
 * Rejects with Auth0's error, or with "No access token" when Auth0 resolves
 * without one (auth0-spa-js 2.30+ types the result as `string | undefined`).
 */
export function useAccessToken(): () => Promise<string> {
  const { getAccessTokenSilently } = useAuth0();

  return useCallback(async () => {
    const token = await getAccessTokenSilently();
    if (!token) {
      throw new Error('No access token');
    }
    return token;
  }, [getAccessTokenSilently]);
}
