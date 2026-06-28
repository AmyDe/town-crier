import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useApplication } from '../useApplication';
import { SpyApplicationRepository } from './spies/spy-application-repository';
import { fullApplication, permittedWithDecision } from './fixtures/planning-application.fixtures';

describe('useApplication', () => {
  it('fetches the application by composite key on mount', async () => {
    const spy = new SpyApplicationRepository();
    const expected = fullApplication();
    spy.fetchApplicationResult = expected;

    const { result } = renderHook(() => useApplication(spy, '42', '2026/0042/FUL'));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(spy.fetchApplicationCalls).toEqual([
      { authority: '42', name: '2026/0042/FUL' },
    ]);
    expect(result.current.application).toEqual(expected);
    expect(result.current.error).toBeNull();
  });

  it('starts in a loading state when authority and name are present', () => {
    const spy = new SpyApplicationRepository();

    const { result } = renderHook(() => useApplication(spy, '42', '2026/0042/FUL'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.application).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('does not fetch when authority or name is null', async () => {
    const spy = new SpyApplicationRepository();

    const { result } = renderHook(() => useApplication(spy, null, null));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(spy.fetchApplicationCalls).toEqual([]);
    expect(result.current.application).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('sets error when fetch fails', async () => {
    const spy = new SpyApplicationRepository();
    spy.fetchApplicationError = new Error('Network unavailable');

    const { result } = renderHook(() => useApplication(spy, '42', '2026/0042/FUL'));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.application).toBeNull();
  });

  it('refetches when the composite key changes', async () => {
    const spy = new SpyApplicationRepository();
    const first = fullApplication();
    const second = permittedWithDecision();
    let callCount = 0;

    spy.fetchApplication = async (authority, name) => {
      callCount += 1;
      spy.fetchApplicationCalls.push({ authority, name });
      return callCount === 1 ? first : second;
    };

    const { result, rerender } = renderHook(
      ({ name }) => useApplication(spy, '42', name),
      { initialProps: { name: '2026/0042/FUL' } },
    );

    await waitFor(() => {
      expect(result.current.application).toEqual(first);
    });

    rerender({ name: '2026/0099/LBC' });

    await waitFor(() => {
      expect(result.current.application).toEqual(second);
    });
  });
});
