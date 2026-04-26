import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createElement } from 'react';
import type { ReactNode } from 'react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useSavedApplications } from '../useSavedApplications';
import { SpySavedApplicationRepository } from './spies/spy-saved-application-repository';
import {
  savedUndecidedApplication,
  savedPermittedApplication,
} from './fixtures/saved-application.fixtures';

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children);
  };
}

describe('useSavedApplications', () => {
  let spy: SpySavedApplicationRepository;

  beforeEach(() => {
    spy = new SpySavedApplicationRepository();
  });

  it('loads saved applications from the repository', async () => {
    const apps = [savedUndecidedApplication(), savedPermittedApplication()];
    spy.listSavedResult = apps;

    const { result } = renderHook(() => useSavedApplications(spy), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.savedApplications).toHaveLength(2);
    expect(result.current.savedApplications[0]?.applicationUid).toBe('APP-001');
    expect(result.current.savedApplications[1]?.applicationUid).toBe('APP-002');
    expect(result.current.error).toBeNull();
  });

  it('exposes loading state while fetching', () => {
    const { result } = renderHook(() => useSavedApplications(spy), {
      wrapper: createWrapper(),
    });

    expect(result.current.isLoading).toBe(true);
    expect(result.current.savedApplications).toEqual([]);
  });

  it('exposes error when fetch fails', async () => {
    spy.listSavedError = new Error('Network unavailable');

    const { result } = renderHook(() => useSavedApplications(spy), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.savedApplications).toEqual([]);
  });

  it('removes a saved application and refetches', async () => {
    const apps = [savedUndecidedApplication(), savedPermittedApplication()];
    spy.listSavedResult = apps;

    const { result } = renderHook(() => useSavedApplications(spy), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.savedApplications).toHaveLength(2);
    });

    // After remove, the list refetches with only one app
    spy.listSavedResult = [savedPermittedApplication()];
    result.current.remove(apps[0]!.applicationUid);

    await waitFor(() => {
      expect(spy.removeCalls).toHaveLength(1);
    });

    expect(spy.removeCalls[0]).toBe('APP-001');

    await waitFor(() => {
      expect(result.current.savedApplications).toHaveLength(1);
    });
  });
});
