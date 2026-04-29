import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useApplications } from '../useApplications';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import { SpySavedApplicationRepository } from './spies/spy-saved-application-repository';
import {
  undecidedApplication,
  permittedApplication,
  rejectedApplication,
} from '../../../components/ApplicationCard/__tests__/fixtures/planning-application-summary.fixtures';
import { savedUndecidedApplication, savedPermittedApplication } from './fixtures/saved-application.fixtures';
import { cambridgeZone, oxfordZone } from './fixtures/zone.fixtures';
import { asApplicationUid, type WatchZoneSummary } from '../../../domain/types';

function makeOptions(overrides?: {
  browsePort?: SpyApplicationsBrowsePort;
  savedRepository?: SpySavedApplicationRepository;
  zones?: readonly WatchZoneSummary[];
}) {
  return {
    browsePort: overrides?.browsePort ?? new SpyApplicationsBrowsePort(),
    savedRepository: overrides?.savedRepository ?? new SpySavedApplicationRepository(),
    zones: overrides?.zones ?? [],
  };
}

describe('useApplications — initial selection', () => {
  it('starts with no selection when zones are empty', () => {
    const { result } = renderHook(() => useApplications(makeOptions()));

    expect(result.current.selectedZone).toBeNull();
    expect(result.current.isAllZonesSelected).toBe(false);
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
    });
    expect(result.current.applications).toHaveLength(1);
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

describe("useApplications — 'All' selection", () => {
  it("selects the synthetic 'All' option and clears applications when saved is off", async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication()];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(1));

    act(() => result.current.selectAllZones());

    expect(result.current.isAllZonesSelected).toBe(true);
    expect(result.current.selectedZone).toBeNull();
    expect(result.current.applications).toEqual([]);
    expect(result.current.isSavedFilterActive).toBe(false);
  });
});

describe('useApplications — Saved filter', () => {
  it('intersects zone applications with saved set when activated on a real zone', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication(), permittedApplication()];
    const savedRepository = new SpySavedApplicationRepository();
    savedRepository.listSavedResult = [
      savedUndecidedApplication({ applicationUid: asApplicationUid('APP-001') }),
      // APP-002 (permitted) NOT saved
    ];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, savedRepository, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(2));

    await act(async () => {
      await result.current.activateSavedFilter();
    });

    expect(result.current.isSavedFilterActive).toBe(true);
    expect(result.current.applications).toHaveLength(1);
    expect(result.current.applications[0]?.uid).toBe('APP-001');
  });

  it("populates applications from saved payloads when 'All' is selected", async () => {
    const savedRepository = new SpySavedApplicationRepository();
    savedRepository.listSavedResult = [
      savedUndecidedApplication(),
      savedPermittedApplication(),
    ];
    const zones = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, savedRepository, zones })),
    );

    await waitFor(() => expect(result.current.selectedZone).not.toBeNull());

    act(() => result.current.selectAllZones());

    await act(async () => {
      await result.current.activateSavedFilter();
    });

    expect(result.current.isAllZonesSelected).toBe(true);
    expect(result.current.isSavedFilterActive).toBe(true);
    expect(result.current.applications).toHaveLength(2);
  });

  it('deactivates saved filter and restores zone applications', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication(), permittedApplication()];
    const savedRepository = new SpySavedApplicationRepository();
    savedRepository.listSavedResult = [savedUndecidedApplication()];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, savedRepository, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(2));

    await act(async () => {
      await result.current.activateSavedFilter();
    });
    expect(result.current.applications).toHaveLength(1);

    act(() => result.current.deactivateSavedFilter());

    expect(result.current.isSavedFilterActive).toBe(false);
    expect(result.current.applications).toHaveLength(2);
  });

  it("clears applications when deactivating saved filter under 'All' selection", async () => {
    const savedRepository = new SpySavedApplicationRepository();
    savedRepository.listSavedResult = [savedUndecidedApplication()];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ savedRepository, zones })),
    );

    await waitFor(() => expect(result.current.selectedZone).not.toBeNull());

    act(() => result.current.selectAllZones());
    await act(async () => {
      await result.current.activateSavedFilter();
    });
    expect(result.current.applications).toHaveLength(1);

    act(() => result.current.deactivateSavedFilter());

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

  it('clears the saved filter when a status is selected', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication()];
    const savedRepository = new SpySavedApplicationRepository();
    savedRepository.listSavedResult = [savedUndecidedApplication()];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, savedRepository, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(1));

    await act(async () => {
      await result.current.activateSavedFilter();
    });
    expect(result.current.isSavedFilterActive).toBe(true);

    act(() => result.current.setStatusFilter('Undecided'));

    expect(result.current.selectedStatusFilter).toBe('Undecided');
    expect(result.current.isSavedFilterActive).toBe(false);
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
