import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useSavedApplications } from '../useSavedApplications';
import { SpySavedApplicationRepository } from '../../Applications/__tests__/spies/spy-saved-application-repository';
import {
  savedUndecidedApplication,
  savedPermittedApplication,
} from '../../Applications/__tests__/fixtures/saved-application.fixtures';
import { asApplicationUid } from '../../../domain/types';

describe('useSavedApplications — load and sort', () => {
  it('loads saved applications and sorts by savedAt desc', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [
      savedUndecidedApplication({
        applicationUid: asApplicationUid('A'),
        savedAt: '2026-01-01T00:00:00Z',
      }),
      savedPermittedApplication({
        applicationUid: asApplicationUid('B'),
        savedAt: '2026-02-01T00:00:00Z',
      }),
    ];

    const { result } = renderHook(() =>
      useSavedApplications({ savedRepository: repo }),
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.applications.map((a) => a.uid)).toEqual(['B', 'A']);
    expect(result.current.error).toBeNull();
  });

  it('returns an empty list when the repository returns no saves', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [];

    const { result } = renderHook(() =>
      useSavedApplications({ savedRepository: repo }),
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.applications).toEqual([]);
    expect(result.current.error).toBeNull();
  });
});

describe('useSavedApplications — status filter', () => {
  it('filters applications by selected status', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [
      savedUndecidedApplication({
        applicationUid: asApplicationUid('A'),
        savedAt: '2026-01-01T00:00:00Z',
      }),
      savedPermittedApplication({
        applicationUid: asApplicationUid('B'),
        savedAt: '2026-02-01T00:00:00Z',
      }),
    ];

    const { result } = renderHook(() =>
      useSavedApplications({ savedRepository: repo }),
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => result.current.setStatusFilter('Undecided'));

    expect(result.current.selectedStatusFilter).toBe('Undecided');
    expect(result.current.applications.map((a) => a.uid)).toEqual(['A']);
  });

  it('returns to the full list when the status filter is cleared', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [
      savedUndecidedApplication({
        applicationUid: asApplicationUid('A'),
        savedAt: '2026-01-01T00:00:00Z',
      }),
      savedPermittedApplication({
        applicationUid: asApplicationUid('B'),
        savedAt: '2026-02-01T00:00:00Z',
      }),
    ];

    const { result } = renderHook(() =>
      useSavedApplications({ savedRepository: repo }),
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    act(() => result.current.setStatusFilter('Permitted'));
    expect(result.current.applications).toHaveLength(1);

    act(() => result.current.setStatusFilter(null));

    expect(result.current.selectedStatusFilter).toBeNull();
    expect(result.current.applications).toHaveLength(2);
  });
});

describe('useSavedApplications — error handling', () => {
  it('captures the repository error and returns an empty list', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedError = new Error('Network unavailable');

    const { result } = renderHook(() =>
      useSavedApplications({ savedRepository: repo }),
    );

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.applications).toEqual([]);
  });
});
