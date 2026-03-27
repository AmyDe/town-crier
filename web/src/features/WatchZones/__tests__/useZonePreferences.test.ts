import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useZonePreferences } from '../useZonePreferences';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { zonePreferences } from './fixtures/watch-zone.fixtures';

describe('useZonePreferences', () => {
  let spy: SpyWatchZoneRepository;

  beforeEach(() => {
    spy = new SpyWatchZoneRepository();
  });

  it('loads preferences for a zone on mount', async () => {
    spy.getPreferencesResult = zonePreferences();

    const { result } = renderHook(() => useZonePreferences(spy, 'zone-1'));

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.preferences).toEqual(zonePreferences());
    expect(result.current.error).toBeNull();
    expect(spy.getPreferencesCalls).toEqual(['zone-1']);
  });

  it('sets error when fetch fails', async () => {
    spy.getPreferencesError = new Error('Not found');

    const { result } = renderHook(() => useZonePreferences(spy, 'zone-1'));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Not found');
    expect(result.current.preferences).toBeNull();
  });

  it('updates preferences and refreshes', async () => {
    spy.getPreferencesResult = zonePreferences();

    const { result } = renderHook(() => useZonePreferences(spy, 'zone-1'));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    const updatedPrefs = zonePreferences({ decisionUpdates: true });
    spy.getPreferencesResult = updatedPrefs;

    await act(async () => {
      await result.current.updatePreferences({
        newApplications: true,
        statusChanges: true,
        decisionUpdates: true,
      });
    });

    expect(spy.updatePreferencesCalls).toEqual([{
      zoneId: 'zone-1',
      data: {
        newApplications: true,
        statusChanges: true,
        decisionUpdates: true,
      },
    }]);
    expect(result.current.preferences?.decisionUpdates).toBe(true);
  });

  it('sets error when update fails', async () => {
    spy.getPreferencesResult = zonePreferences();

    const { result } = renderHook(() => useZonePreferences(spy, 'zone-1'));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    spy.updatePreferencesError = new Error('Update failed');

    await act(async () => {
      await result.current.updatePreferences({
        newApplications: false,
        statusChanges: false,
        decisionUpdates: false,
      });
    });

    expect(result.current.error).toBe('Update failed');
  });

  it('tracks saving state during update', async () => {
    spy.getPreferencesResult = zonePreferences();

    const { result } = renderHook(() => useZonePreferences(spy, 'zone-1'));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.isSaving).toBe(false);

    await act(async () => {
      await result.current.updatePreferences({
        newApplications: true,
        statusChanges: true,
        decisionUpdates: true,
      });
    });

    expect(result.current.isSaving).toBe(false);
  });
});
