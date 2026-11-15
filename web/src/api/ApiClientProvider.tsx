import type { ReactNode } from 'react';
import { useMemo } from 'react';
import { useAccessToken } from '../auth/useAccessToken';
import { createApiClient } from './client';
import { ApiClientProvider as ApiClientCtxProvider } from './useApiClient';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

interface Props {
  children: ReactNode;
}

export function ApiClientProvider({ children }: Props) {
  const getAccessToken = useAccessToken();

  const client = useMemo(
    () => createApiClient(API_BASE_URL, getAccessToken),
    [getAccessToken],
  );

  return (
    <ApiClientCtxProvider value={client}>
      {children}
    </ApiClientCtxProvider>
  );
}
