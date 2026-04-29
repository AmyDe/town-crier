import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useApplications } from '../useApplications';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import {
  undecidedApplication,
  permittedApplication,
  rejectedApplication,
} from '../../../components/ApplicationCard/__tests__/fixtures/planning-application-summary.fixtures';
import { cambridgeZone, oxfordZone } from './fixtures/zone.fixtures';
import type { WatchZoneSummary } from '../../../domain/types';

function makeOptions(overrides?: {
  browsePort?: SpyApplicationsBrowsePort;
  zones?: readonly WatchZoneSummary[];
}) {
  return {
    browsePort: overrides?.browsePort ?? new SpyApplicationsBrowsePort(),
    zones: overrides?.zones ?? [],
  };
}

describe('useApplications — initial selection', () => {
  it('starts with no selection when zones are empty', () => {
    const { result } = renderHook(() => useApplications(makeOptions()));

    expect(result.current.selectedZone).toBeNull();
    expect(result.current.applications).toEqual([]);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('auto-selects the first zone when zones become available', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication()];
    const zones = [cambridgeZone(), oxfordZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => {
      expect(result.current.selectedZone).toEqual(cambridgeZone());
      expect(result.current.applications).toHaveLength(1);
    });
    expect(browsePort.fetchByZoneCalls).toEqual([cambridgeZone().id]);
  });
});

describe('useApplications — selecting a zone', () => {
  it('fetches applications when a zone is selected', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication(), permittedApplication()];
    const zones = [cambridgeZone(), oxfordZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(2));

    act(() => result.current.selectZone(oxfordZone()));

    await waitFor(() => expect(result.current.selectedZone).toEqual(oxfordZone()));
    expect(browsePort.fetchByZoneCalls).toContain(oxfordZone().id);
  });

  it('sets error when fetch fails', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneError = new Error('Network unavailable');
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.error).not.toBeNull());
    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.applications).toEqual([]);
  });
});

describe('useApplications — status filter', () => {
  it('filters applications by selected status', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [
      undecidedApplication(),
      permittedApplication(),
      rejectedApplication(),
    ];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(3));

    act(() => result.current.setStatusFilter('Permitted'));

    expect(result.current.selectedStatusFilter).toBe('Permitted');
    expect(result.current.applications).toHaveLength(1);
    expect(result.current.applications[0]?.appState).toBe('Permitted');
  });

  it('returns to unfiltered list when status is cleared', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication(), permittedApplication()];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(2));

    act(() => result.current.setStatusFilter('Permitted'));
    expect(result.current.applications).toHaveLength(1);

    act(() => result.current.setStatusFilter(null));

    expect(result.current.selectedStatusFilter).toBeNull();
    expect(result.current.applications).toHaveLength(2);
  });
});
