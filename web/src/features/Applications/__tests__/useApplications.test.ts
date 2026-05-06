import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useApplications } from '../useApplications';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import { SpyNotificationStateRepository } from './spies/spy-notification-state-repository';
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
  notificationStateRepository?: SpyNotificationStateRepository;
}) {
  return {
    browsePort: overrides?.browsePort ?? new SpyApplicationsBrowsePort(),
    zones: overrides?.zones ?? [],
    notificationStateRepository:
      overrides?.notificationStateRepository ?? new SpyNotificationStateRepository(),
  };
}

beforeEach(() => {
  window.localStorage.clear();
});

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

describe('useApplications — unread chip count', () => {
  it('derives unreadCount from the count of distinct applications with a latestUnreadEvent in the active zone', async () => {
    // Three applications, two of which carry an unread event. The chip must
    // report **two** (distinct apps), not a server-side event count — fixing
    // the regression where multi-event apps inflated the chip beyond the
    // visible row count (tc-u6bm).
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [
      undecidedApplication({
        latestUnreadEvent: {
          type: 'NewApplication',
          decision: null,
          createdAt: '2026-04-01T00:00:00Z',
        },
      }),
      permittedApplication(), // null
      rejectedApplication({
        latestUnreadEvent: {
          type: 'DecisionUpdate',
          decision: 'Rejected',
          createdAt: '2026-04-15T00:00:00Z',
        },
      }),
    ];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(3));
    expect(result.current.unreadCount).toBe(2);
  });

  it('does not call notificationStateRepository.getState — chip count is derived client-side', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const stateRepo = new SpyNotificationStateRepository();
    const zones = [cambridgeZone()];

    renderHook(() =>
      useApplications(
        makeOptions({ browsePort, zones, notificationStateRepository: stateRepo }),
      ),
    );

    // Allow any pending effects to flush. The hook must not reach for the
    // notification-state snapshot purely for the chip count.
    await waitFor(() => expect(browsePort.fetchByZoneCalls.length).toBe(1));
    expect(stateRepo.getStateCalls).toBe(0);
  });

  it('reports 0 unread when no applications carry a latestUnreadEvent', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication(), permittedApplication()];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(2));
    expect(result.current.unreadCount).toBe(0);
  });
});

describe('useApplications — Unread filter', () => {
  it('keeps only rows with a non-null latestUnreadEvent when Unread is selected', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [
      undecidedApplication({
        latestUnreadEvent: {
          type: 'NewApplication',
          decision: null,
          createdAt: '2026-04-01T00:00:00Z',
        },
      }),
      permittedApplication(), // null
      rejectedApplication({
        latestUnreadEvent: {
          type: 'DecisionUpdate',
          decision: 'Rejected',
          createdAt: '2026-04-15T00:00:00Z',
        },
      }),
    ];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(3));

    act(() => result.current.setUnreadOnly(true));

    expect(result.current.unreadOnly).toBe(true);
    expect(result.current.applications).toHaveLength(2);
    // Default sort is recent-activity (desc) — rejected event 2026-04-15
    // outranks undecided 2026-04-01.
    expect(result.current.applications.map((a) => a.uid)).toEqual([
      rejectedApplication().uid,
      undecidedApplication().uid,
    ]);
  });

  it('clears the status filter when Unread is selected (single-select chip group)', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.selectedZone).not.toBeNull());

    act(() => result.current.setStatusFilter('Permitted'));
    expect(result.current.selectedStatusFilter).toBe('Permitted');

    act(() => result.current.setUnreadOnly(true));

    expect(result.current.unreadOnly).toBe(true);
    expect(result.current.selectedStatusFilter).toBeNull();
  });
});

describe('useApplications — markAllRead', () => {
  it('calls the repository, optimistically zeroes unreadCount, and refetches the row data', async () => {
    // Initial load has two unread apps; the post-mark refetch returns the
    // same apps with `latestUnreadEvent: null`, so the derived chip count
    // collapses to zero without any server-side count fetch.
    const browsePort = new SpyApplicationsBrowsePort();
    const initialApps = [
      undecidedApplication({
        latestUnreadEvent: {
          type: 'NewApplication',
          decision: null,
          createdAt: '2026-04-01T00:00:00Z',
        },
      }),
      permittedApplication({
        latestUnreadEvent: {
          type: 'DecisionUpdate',
          decision: 'Permitted',
          createdAt: '2026-04-15T00:00:00Z',
        },
      }),
    ];
    browsePort.fetchByZoneResult = initialApps;
    const stateRepo = new SpyNotificationStateRepository();
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(
        makeOptions({ browsePort, zones, notificationStateRepository: stateRepo }),
      ),
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(2));
    const initialFetchCount = browsePort.fetchByZoneCalls.length;

    // Simulate the post-mark refetch returning rows with no unread events.
    browsePort.fetchByZoneResult = [
      undecidedApplication({ latestUnreadEvent: null }),
      permittedApplication({ latestUnreadEvent: null }),
    ];

    await act(async () => {
      await result.current.markAllRead();
    });

    expect(stateRepo.markAllReadCalls).toBe(1);
    expect(result.current.unreadCount).toBe(0);
    // After mark-all-read every row's latestUnreadEvent must drop to null —
    // simplest correct implementation is to refetch the zone applications.
    expect(browsePort.fetchByZoneCalls.length).toBeGreaterThan(initialFetchCount);
  });
});

describe('useApplications — sort', () => {
  it('exposes the recent-activity option as the default sort', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    expect(result.current.sort).toBe('recent-activity');
  });

  it('orders rows by max(receivedDate, latestUnreadEvent.createdAt) desc when sort is recent-activity', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    // App A — receivedDate 2026-01-01, no unread → 2026-01-01
    // App B — receivedDate 2026-02-01, no unread → 2026-02-01
    // App C — receivedDate 2026-01-15, unread 2026-04-01 → 2026-04-01 (newest)
    browsePort.fetchByZoneResult = [
      undecidedApplication({
        uid: undecidedApplication().uid,
        startDate: '2026-01-01',
      }),
      permittedApplication({ startDate: '2026-02-01' }),
      rejectedApplication({
        startDate: '2026-01-15',
        latestUnreadEvent: {
          type: 'DecisionUpdate',
          decision: 'Rejected',
          createdAt: '2026-04-01T00:00:00Z',
        },
      }),
    ];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(3));

    expect(result.current.applications.map((a) => a.uid)).toEqual([
      rejectedApplication().uid, // 2026-04-01
      permittedApplication().uid, // 2026-02-01
      undecidedApplication().uid, // 2026-01-01
    ]);
  });

  it('orders rows by startDate desc when sort is newest', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [
      undecidedApplication({ startDate: '2026-01-01' }),
      permittedApplication({ startDate: '2026-03-01' }),
      rejectedApplication({ startDate: '2026-02-01' }),
    ];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(3));

    act(() => result.current.setSort('newest'));

    expect(result.current.applications.map((a) => a.uid)).toEqual([
      permittedApplication().uid,
      rejectedApplication().uid,
      undecidedApplication().uid,
    ]);
  });

  it('orders rows by startDate asc when sort is oldest', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [
      undecidedApplication({ startDate: '2026-03-01' }),
      permittedApplication({ startDate: '2026-01-01' }),
      rejectedApplication({ startDate: '2026-02-01' }),
    ];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(3));

    act(() => result.current.setSort('oldest'));

    expect(result.current.applications.map((a) => a.uid)).toEqual([
      permittedApplication().uid,
      rejectedApplication().uid,
      undecidedApplication().uid,
    ]);
  });

  it('orders rows by appState alphabetically when sort is status', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [
      undecidedApplication(),
      permittedApplication(),
      rejectedApplication(),
    ];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(3));

    act(() => result.current.setSort('status'));

    // Expected alphabetical order: Permitted, Rejected, Undecided
    expect(result.current.applications.map((a) => a.appState)).toEqual([
      'Permitted',
      'Rejected',
      'Undecided',
    ]);
  });

  it('persists the sort selection to localStorage under applicationsListSort', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    act(() => result.current.setSort('newest'));

    expect(window.localStorage.getItem('applicationsListSort')).toBe('newest');
  });

  it('rehydrates the persisted sort selection from localStorage on mount', async () => {
    window.localStorage.setItem('applicationsListSort', 'oldest');
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    expect(result.current.sort).toBe('oldest');
  });

  it('falls back to recent-activity when localStorage holds an unknown value', async () => {
    window.localStorage.setItem('applicationsListSort', 'unsupported-key');
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    expect(result.current.sort).toBe('recent-activity');
  });
});

describe('useApplications — distance sort', () => {
  // Cambridge city centre. Test fixtures are placed at known offsets so the
  // expected ordering is independent of haversine constants.
  const cambridgeCentre = { latitude: 52.2053, longitude: 0.1218 };

  function withLocation(
    base: ReturnType<typeof undecidedApplication>,
    location: { latitude: number; longitude: number } | null,
  ) {
    return {
      ...base,
      latitude: location?.latitude ?? null,
      longitude: location?.longitude ?? null,
    };
  }

  it('orders rows by ascending haversine distance from the selected zone centre', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    // Zone centre is Cambridge (52.2053, 0.1218).
    // far    — Oxford (~107 km west)
    // mid    — Royston (~ 17 km south-west)
    // close  — 200 m east of the centre
    const far = withLocation(undecidedApplication({ uid: undecidedApplication().uid }), {
      latitude: 51.752,
      longitude: -1.2577,
    });
    const mid = withLocation(permittedApplication(), {
      latitude: 52.0497,
      longitude: -0.0258,
    });
    const close = withLocation(rejectedApplication(), {
      latitude: 52.2053,
      longitude: 0.1247,
    });
    browsePort.fetchByZoneResult = [far, mid, close];
    const zones = [cambridgeZone({ latitude: cambridgeCentre.latitude, longitude: cambridgeCentre.longitude })];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(3));

    act(() => result.current.setSort('distance'));

    expect(result.current.applications.map((a) => a.uid)).toEqual([
      close.uid,
      mid.uid,
      far.uid,
    ]);
  });

  it('places applications without a location at the end of the distance sort', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    const located = withLocation(permittedApplication(), {
      latitude: 52.2053,
      longitude: 0.13,
    });
    const unlocated = withLocation(rejectedApplication(), null);
    browsePort.fetchByZoneResult = [unlocated, located];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(2));

    act(() => result.current.setSort('distance'));

    expect(result.current.applications.map((a) => a.uid)).toEqual([
      located.uid,
      unlocated.uid,
    ]);
  });

  it('preserves incoming order between multiple unlocated rows (stable tiebreak)', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    const a = withLocation(undecidedApplication(), null);
    const b = withLocation(permittedApplication(), null);
    const c = withLocation(rejectedApplication(), null);
    browsePort.fetchByZoneResult = [a, b, c];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.applications).toHaveLength(3));

    act(() => result.current.setSort('distance'));

    expect(result.current.applications.map((a) => a.uid)).toEqual([
      a.uid,
      b.uid,
      c.uid,
    ]);
  });

  it('persists "distance" to localStorage under applicationsListSort', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    act(() => result.current.setSort('distance'));

    expect(window.localStorage.getItem('applicationsListSort')).toBe('distance');
  });

  it('rehydrates "distance" from localStorage on mount when a zone is available', async () => {
    window.localStorage.setItem('applicationsListSort', 'distance');
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    expect(result.current.sort).toBe('distance');
  });
});

describe('useApplications — availableSortOptions', () => {
  it('omits "distance" when no zone is selected (multi-zone view)', () => {
    const { result } = renderHook(() => useApplications(makeOptions()));

    expect(result.current.selectedZone).toBeNull();
    expect(result.current.availableSortOptions).not.toContain('distance');
  });

  it('includes "distance" once a zone has been auto-selected', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones })),
    );

    await waitFor(() => expect(result.current.selectedZone).not.toBeNull());

    expect(result.current.availableSortOptions).toContain('distance');
  });
});
