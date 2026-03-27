import type { ReactNode } from 'react';
import { useMemo } from 'react';
import { useAuth0 } from '@auth0/auth0-react';
import { createApiClient } from './client';
import { ApiClientContext } from './useApiClient';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

interface Props {
  children: ReactNode;
}

export function ApiClientProvider({ children }: Props) {
  const { getAccessTokenSilently } = useAuth0();

  const client = useMemo(
    () => createApiClient(API_BASE_URL, getAccessTokenSilently),
    [getAccessTokenSilently],
  );

  return (
    <ApiClientContext.Provider value={client}>
      {children}
    </ApiClientContext.Provider>
  );
}
