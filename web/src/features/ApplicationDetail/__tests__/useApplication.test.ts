import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useApplication } from '../useApplication';
import { SpyApplicationRepository } from './spies/spy-application-repository';
import { fullApplication, permittedWithDecision } from './fixtures/planning-application.fixtures';
import { asApplicationUid } from '../../../domain/types';

describe('useApplication', () => {
  it('fetches the application on mount', async () => {
    const spy = new SpyApplicationRepository();
    const expected = fullApplication();
    spy.fetchApplicationResult = expected;
    const uid = asApplicationUid('APP-001');

    const { result } = renderHook(() => useApplication(spy, uid));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(spy.fetchApplicationCalls).toEqual([uid]);
    expect(result.current.application).toEqual(expected);
    expect(result.current.error).toBeNull();
  });

  it('starts in a loading state', () => {
    const spy = new SpyApplicationRepository();
    const uid = asApplicationUid('APP-001');

    const { result } = renderHook(() => useApplication(spy, uid));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.application).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('sets error when fetch fails', async () => {
    const spy = new SpyApplicationRepository();
    spy.fetchApplicationError = new Error('Network unavailable');
    const uid = asApplicationUid('APP-001');

    const { result } = renderHook(() => useApplication(spy, uid));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.application).toBeNull();
  });

  it('refetches when uid changes', async () => {
    const spy = new SpyApplicationRepository();
    const first = fullApplication();
    const second = permittedWithDecision();
    let callCount = 0;

    const originalFetch = spy.fetchApplication.bind(spy);
    spy.fetchApplication = async (uid) => {
      callCount += 1;
      spy.fetchApplicationResult = callCount === 1 ? first : second;
      return originalFetch(uid);
    };

    const uid1 = asApplicationUid('APP-001');
    const uid2 = asApplicationUid('APP-002');

    const { result, rerender } = renderHook(
      ({ uid }) => useApplication(spy, uid),
      { initialProps: { uid: uid1 } },
    );

    await waitFor(() => {
      expect(result.current.application).toEqual(first);
    });

    rerender({ uid: uid2 });

    await waitFor(() => {
      expect(result.current.application).toEqual(second);
    });
  });
});
