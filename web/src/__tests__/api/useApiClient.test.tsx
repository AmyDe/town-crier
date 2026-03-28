import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { ApiClientProvider, useApiClient } from '../../api/useApiClient';
import type { ApiClient } from '../../api/client';

const fakeClient: ApiClient = {
  get: async () => ({}) as never,
  post: async () => ({}) as never,
  put: async () => {},
  patch: async () => ({}) as never,
  delete: async () => {},
};

describe('useApiClient', () => {
  it('returns the ApiClient from context', () => {
    function wrapper({ children }: { children: ReactNode }) {
      return (
        <ApiClientProvider value={fakeClient}>
          {children}
        </ApiClientProvider>
      );
    }
    const { result } = renderHook(() => useApiClient(), { wrapper });
    expect(result.current).toBe(fakeClient);
  });

  it('throws when used outside provider', () => {
    expect(() => {
      renderHook(() => useApiClient());
    }).toThrow('useApiClient must be used within a ApiClient.Provider');
  });
});
