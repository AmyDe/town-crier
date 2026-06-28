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
import { asApplicationUid } from '../../../domain/types';
import type {
  PlanningApplicationSummary,
  WatchZoneSummary,
} from '../../../domain/types';

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

function withUid(
  base: PlanningApplicationSummary,
  uid: string,
): PlanningApplicationSummary {
  return { ...base, uid: asApplicationUid(uid) };
}

beforeEach(() => {
  window.localStorage.clear();
});

describe('useApplications — initial selection', () => {
  it('starts with no selection and issues no query when zones are empty', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    const { result } = renderHook(() => useApplications(makeOptions({ browsePort })));

    expect(result.current.selectedZone).toBeNull();
    expect(result.current.applications).toEqual([]);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(browsePort.fetchByZoneCalls).toEqual([]);
  });

  it('auto-selects the first zone and fetches its first page (cursor null, default sort)', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = { rows: [undecidedApplication()], nextCursor: null };
    const zones = [cambridgeZone(), oxfordZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => {
      expect(result.current.selectedZone).toEqual(cambridgeZone());
      expect(result.current.applications).toHaveLength(1);
    });
    expect(browsePort.fetchByZoneCalls).toHaveLength(1);
    expect(browsePort.fetchByZoneCalls[0]).toEqual({
      zoneId: cambridgeZone().id,
      sort: 'recent-activity',
      status: null,
      unread: false,
      cursor: null,
    });
  });
});

describe('useApplications — selecting a zone', () => {
  it('fetches the newly selected zone (page 1)', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = {
      rows: [undecidedApplication(), permittedApplication()],
      nextCursor: null,
    };
    const zones = [cambridgeZone(), oxfordZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(2));

    act(() => result.current.selectZone(oxfordZone()));

    await waitFor(() => expect(result.current.selectedZone).toEqual(oxfordZone()));
    expect(browsePort.fetchByZoneCalls.some((c) => c.zoneId === oxfordZone().id)).toBe(true);
  });

  it('sets error when the fetch fails', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneError = new Error('Network unavailable');
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.error).not.toBeNull());
    expect(result.current.error).toBe('Network unavailable');
    expect(result.current.applications).toEqual([]);
  });
});

describe('useApplications — server-driven rendering (no client sort/filter)', () => {
  it('renders rows in the exact order the server returned them', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    const a = withUid(undecidedApplication({ startDate: '2026-01-01' }), 'A');
    const b = withUid(permittedApplication({ startDate: '2026-12-01' }), 'B');
    const c = withUid(rejectedApplication({ startDate: '2026-06-01' }), 'C');
    // Deliberately not in any natural order — the hook must not re-sort.
    browsePort.fetchByZoneResult = { rows: [b, a, c], nextCursor: null };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(3));
    expect(result.current.applications.map((x) => x.uid)).toEqual([b.uid, a.uid, c.uid]);
  });

  it('sends the selected sort to the server and resets to page 1 (cursor null)', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = { rows: [undecidedApplication()], nextCursor: null };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(1));

    act(() => result.current.setSort('oldest'));

    await waitFor(() => expect(browsePort.fetchByZoneCalls.at(-1)!.sort).toBe('oldest'));
    expect(browsePort.fetchByZoneCalls.at(-1)!.cursor).toBeNull();
  });
});

describe('useApplications — status filter (server-driven)', () => {
  it('sends ?status to the server and renders the returned rows verbatim', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResponder = (q) => {
      const all = [undecidedApplication(), permittedApplication(), rejectedApplication()];
      const rows = q.status === null ? all : all.filter((a) => a.appState === q.status);
      return { rows, nextCursor: null };
    };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(3));

    act(() => result.current.setStatusFilter('Permitted'));

    await waitFor(() => expect(result.current.applications).toHaveLength(1));
    expect(result.current.applications[0]?.appState).toBe('Permitted');
    const lastCall = browsePort.fetchByZoneCalls.at(-1)!;
    expect(lastCall.status).toBe('Permitted');
    expect(lastCall.cursor).toBeNull();
  });

  it('returns to an unfiltered query when the status is cleared', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResponder = (q) => {
      const all = [undecidedApplication(), permittedApplication()];
      const rows = q.status === null ? all : all.filter((a) => a.appState === q.status);
      return { rows, nextCursor: null };
    };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(2));
    act(() => result.current.setStatusFilter('Permitted'));
    await waitFor(() => expect(result.current.applications).toHaveLength(1));

    act(() => result.current.setStatusFilter(null));

    await waitFor(() => expect(result.current.applications).toHaveLength(2));
    expect(browsePort.fetchByZoneCalls.at(-1)!.status).toBeNull();
  });
});

describe('useApplications — unread filter (server-driven, mutually exclusive)', () => {
  it('sends ?unread=true and never sends a status alongside it', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResponder = (q) => {
      const unread = undecidedApplication({
        latestUnreadEvent: { type: 'NewApplication', decision: null, createdAt: '2026-04-01T00:00:00Z' },
      });
      const read = permittedApplication();
      const rows = q.unread ? [unread] : [unread, read];
      return { rows, nextCursor: null };
    };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(2));

    act(() => result.current.setUnreadOnly(true));

    await waitFor(() => expect(result.current.applications).toHaveLength(1));
    const lastCall = browsePort.fetchByZoneCalls.at(-1)!;
    expect(lastCall.unread).toBe(true);
    expect(lastCall.status).toBeNull();
  });

  it('never sends status and unread together across the whole session', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = { rows: [], nextCursor: null };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));
    await waitFor(() => expect(result.current.selectedZone).not.toBeNull());

    act(() => result.current.setStatusFilter('Permitted'));
    await waitFor(() => expect(browsePort.fetchByZoneCalls.at(-1)!.status).toBe('Permitted'));
    act(() => result.current.setUnreadOnly(true));
    await waitFor(() => expect(browsePort.fetchByZoneCalls.at(-1)!.unread).toBe(true));

    for (const call of browsePort.fetchByZoneCalls) {
      expect(call.status !== null && call.unread).toBe(false);
    }
  });

  it('clears the status filter when unread is selected (single-select group)', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = { rows: [], nextCursor: null };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));
    await waitFor(() => expect(result.current.selectedZone).not.toBeNull());

    act(() => result.current.setStatusFilter('Permitted'));
    expect(result.current.selectedStatusFilter).toBe('Permitted');

    act(() => result.current.setUnreadOnly(true));

    expect(result.current.unreadOnly).toBe(true);
    expect(result.current.selectedStatusFilter).toBeNull();
  });
});

describe('useApplications — keyset pagination (X-Next-Cursor)', () => {
  it('follows the cursor across pages until it is absent, appending each page (large zone)', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    const p1 = withUid(undecidedApplication(), 'P1');
    const p2 = withUid(permittedApplication(), 'P2');
    const p3 = withUid(rejectedApplication(), 'P3');
    browsePort.fetchByZoneResponder = (q) => {
      if (q.cursor === null) return { rows: [p1], nextCursor: 'c1' };
      if (q.cursor === 'c1') return { rows: [p2], nextCursor: 'c2' };
      if (q.cursor === 'c2') return { rows: [p3], nextCursor: null };
      throw new Error(`unexpected cursor ${String(q.cursor)}`);
    };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(1));
    expect(result.current.hasMore).toBe(true);

    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.applications).toHaveLength(2));
    expect(result.current.hasMore).toBe(true);

    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.applications).toHaveLength(3));
    expect(result.current.hasMore).toBe(false);
    expect(result.current.applications.map((a) => a.uid)).toEqual([p1.uid, p2.uid, p3.uid]);

    // Paging past the end is a no-op (no extra fetch).
    const callCount = browsePort.fetchByZoneCalls.length;
    act(() => result.current.loadMore());
    expect(browsePort.fetchByZoneCalls.length).toBe(callCount);
  });

  it('forwards the active sort + filter on every load-more page', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResponder = (q) => ({
      rows: [withUid(undecidedApplication(), `${q.sort}-${String(q.cursor)}`)],
      nextCursor: q.cursor === null ? 'c1' : null,
    });
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));
    await waitFor(() => expect(result.current.applications).toHaveLength(1));

    act(() => result.current.setSort('newest'));
    // Wait for the page-1 re-query to settle (so the next cursor is available)
    // before paging — otherwise loadMore races the in-flight first page.
    await waitFor(() =>
      expect(result.current.applications[0]?.uid).toBe(asApplicationUid('newest-null')),
    );
    expect(result.current.hasMore).toBe(true);

    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.applications).toHaveLength(2));
    const lastCall = browsePort.fetchByZoneCalls.at(-1)!;
    expect(lastCall.sort).toBe('newest');
    expect(lastCall.cursor).toBe('c1');
  });

  it('resets pagination to page 1 (cursor cleared, pages discarded) when the sort changes', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResponder = (q) => {
      if (q.cursor === null) return { rows: [withUid(undecidedApplication(), `first-${q.sort}`)], nextCursor: 'more' };
      return { rows: [withUid(permittedApplication(), 'second')], nextCursor: null };
    };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));
    await waitFor(() => expect(result.current.applications).toHaveLength(1));

    act(() => result.current.loadMore());
    await waitFor(() => expect(result.current.applications).toHaveLength(2));

    act(() => result.current.setSort('oldest'));

    // The two accumulated pages are discarded; we are back to a single page-1 row.
    await waitFor(() => expect(result.current.applications).toHaveLength(1));
    const lastCall = browsePort.fetchByZoneCalls.at(-1)!;
    expect(lastCall.sort).toBe('oldest');
    expect(lastCall.cursor).toBeNull();
    expect(result.current.hasMore).toBe(true);
  });
});

describe('useApplications — unread chip count (derived from loaded rows)', () => {
  it('counts distinct applications carrying a latestUnreadEvent', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = {
      rows: [
        undecidedApplication({
          latestUnreadEvent: { type: 'NewApplication', decision: null, createdAt: '2026-04-01T00:00:00Z' },
        }),
        permittedApplication(),
        rejectedApplication({
          latestUnreadEvent: { type: 'DecisionUpdate', decision: 'Rejected', createdAt: '2026-04-15T00:00:00Z' },
        }),
      ],
      nextCursor: null,
    };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.applications).toHaveLength(3));
    expect(result.current.unreadCount).toBe(2);
  });

  it('does not reach for the notification-state snapshot for the chip count', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = { rows: [], nextCursor: null };
    const stateRepo = new SpyNotificationStateRepository();
    const zones = [cambridgeZone()];

    renderHook(() =>
      useApplications(makeOptions({ browsePort, zones, notificationStateRepository: stateRepo })),
    );

    await waitFor(() => expect(browsePort.fetchByZoneCalls.length).toBe(1));
    expect(stateRepo.getStateCalls).toBe(0);
  });
});

describe('useApplications — markAllRead', () => {
  it('marks read on the server then refetches page 1 with the active sort/filter', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = {
      rows: [
        undecidedApplication({
          latestUnreadEvent: { type: 'NewApplication', decision: null, createdAt: '2026-04-01T00:00:00Z' },
        }),
        permittedApplication({
          latestUnreadEvent: { type: 'DecisionUpdate', decision: 'Permitted', createdAt: '2026-04-15T00:00:00Z' },
        }),
      ],
      nextCursor: null,
    };
    const stateRepo = new SpyNotificationStateRepository();
    const zones = [cambridgeZone()];

    const { result } = renderHook(() =>
      useApplications(makeOptions({ browsePort, zones, notificationStateRepository: stateRepo })),
    );

    await waitFor(() => expect(result.current.unreadCount).toBe(2));
    const before = browsePort.fetchByZoneCalls.length;

    browsePort.fetchByZoneResult = {
      rows: [
        undecidedApplication({ latestUnreadEvent: null }),
        permittedApplication({ latestUnreadEvent: null }),
      ],
      nextCursor: null,
    };

    await act(async () => {
      await result.current.markAllRead();
    });

    expect(stateRepo.markAllReadCalls).toBe(1);
    expect(result.current.unreadCount).toBe(0);
    expect(browsePort.fetchByZoneCalls.length).toBeGreaterThan(before);
    expect(browsePort.fetchByZoneCalls.at(-1)!.cursor).toBeNull();
  });
});

describe('useApplications — sort persistence', () => {
  // A stable spy per test — `makeOptions` defaults would mint a fresh port on
  // every render, changing the query effect's dependency identity and looping.
  function persistenceOptions() {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = { rows: [], nextCursor: null };
    return makeOptions({ browsePort, zones: [cambridgeZone()] });
  }

  it('defaults to recent-activity', () => {
    const options = persistenceOptions();
    const { result } = renderHook(() => useApplications(options));
    expect(result.current.sort).toBe('recent-activity');
  });

  it('persists the sort selection to localStorage', () => {
    const options = persistenceOptions();
    const { result } = renderHook(() => useApplications(options));
    act(() => result.current.setSort('newest'));
    expect(window.localStorage.getItem('applicationsListSort')).toBe('newest');
  });

  it('rehydrates the persisted sort on mount', () => {
    window.localStorage.setItem('applicationsListSort', 'oldest');
    const options = persistenceOptions();
    const { result } = renderHook(() => useApplications(options));
    expect(result.current.sort).toBe('oldest');
  });

  it('falls back to recent-activity for an unknown persisted value', () => {
    window.localStorage.setItem('applicationsListSort', 'nonsense');
    const options = persistenceOptions();
    const { result } = renderHook(() => useApplications(options));
    expect(result.current.sort).toBe('recent-activity');
  });
});

describe('useApplications — availableSortOptions', () => {
  it('omits "distance" when no zone is selected', () => {
    const { result } = renderHook(() => useApplications(makeOptions()));
    expect(result.current.selectedZone).toBeNull();
    expect(result.current.availableSortOptions).not.toContain('distance');
  });

  it('includes "distance" once a zone has been auto-selected', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = { rows: [], nextCursor: null };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));

    await waitFor(() => expect(result.current.selectedZone).not.toBeNull());
    expect(result.current.availableSortOptions).toContain('distance');
  });

  it('sends sort=distance to the server when distance is selected', async () => {
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = { rows: [], nextCursor: null };
    const zones = [cambridgeZone()];

    const { result } = renderHook(() => useApplications(makeOptions({ browsePort, zones })));
    await waitFor(() => expect(result.current.selectedZone).not.toBeNull());

    act(() => result.current.setSort('distance'));

    await waitFor(() => expect(browsePort.fetchByZoneCalls.at(-1)!.sort).toBe('distance'));
  });
});
