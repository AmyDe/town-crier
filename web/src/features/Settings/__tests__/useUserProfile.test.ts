import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useUserProfile } from '../useUserProfile';
import { SpySettingsRepository } from './spies/spy-settings-repository';
import { proUserProfile } from './fixtures/user-profile.fixtures';

describe('useUserProfile', () => {
  let spy: SpySettingsRepository;
  let logout: () => void;
  let logoutCalls: number;

  beforeEach(() => {
    spy = new SpySettingsRepository();
    logoutCalls = 0;
    logout = () => { logoutCalls++; };
  });

  it('loads profile on mount', async () => {
    spy.fetchProfileResult = proUserProfile();

    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.profile).toEqual(proUserProfile());
    expect(result.current.error).toBeNull();
    expect(spy.fetchProfileCalls).toBe(1);
  });

  it('sets isLoading to true while fetching', () => {
    const { result } = renderHook(() => useUserProfile(spy, logout));

    expect(result.current.isLoading).toBe(true);
  });

  it('sets error when profile fetch fails', async () => {
    spy.fetchProfileError = new Error('Network error');

    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.profile).toBeNull();
  });

  it('exports data and triggers download', async () => {
    const blobData = JSON.stringify({ userId: 'test' });
    spy.exportDataResult = new Blob([blobData], { type: 'application/json' });

    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.exportData();
    });

    expect(spy.exportDataCalls).toBe(1);
    expect(result.current.isExporting).toBe(false);
  });

  it('sets error when export fails', async () => {
    spy.exportDataError = new Error('Export failed');

    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.exportData();
    });

    expect(result.current.error).toBe('Export failed');
  });

  it('deletes account and calls logout', async () => {
    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.deleteAccount();
    });

    expect(spy.deleteAccountCalls).toBe(1);
    expect(logoutCalls).toBe(1);
  });

  it('sets error when delete fails', async () => {
    spy.deleteAccountError = new Error('Delete failed');

    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.deleteAccount();
    });

    expect(result.current.error).toBe('Delete failed');
    expect(logoutCalls).toBe(0);
  });

  it('sets isDeleting while delete is in progress', async () => {
    let resolveDelete: () => void = () => {};
    spy.deleteAccount = () =>
      new Promise<void>((resolve) => {
        resolveDelete = resolve;
      });

    const { result } = renderHook(() => useUserProfile(spy, logout));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    let deletePromise: Promise<void>;
    act(() => {
      deletePromise = result.current.deleteAccount();
    });

    expect(result.current.isDeleting).toBe(true);

    await act(async () => {
      resolveDelete();
      await deletePromise!;
    });

    expect(result.current.isDeleting).toBe(false);
  });
});
