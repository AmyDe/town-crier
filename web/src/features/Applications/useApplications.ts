import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import type {
  WatchZoneSummary,
  PlanningApplicationSummary,
  ApplicationStatus,
  ApplicationsSort,
} from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { NotificationStateRepository } from '../../domain/ports/notification-state-repository';

/**
 * Re-exported from the domain so existing consumers (`ApplicationsPage`) keep
 * importing the sort type from this hook. The vocabulary itself, and its
 * mapping onto the server's `?sort=` param, lives in `domain/types`.
 */
export type { ApplicationsSort };

const APPLICATIONS_SORT_VALUES: readonly ApplicationsSort[] = [
  'recent-activity',
  'newest',
  'oldest',
  'status',
  'distance',
];

const SORT_STORAGE_KEY = 'applicationsListSort';
const DEFAULT_SORT: ApplicationsSort = 'recent-activity';

function readPersistedSort(): ApplicationsSort {
  try {
    const raw = window.localStorage.getItem(SORT_STORAGE_KEY);
    if (raw !== null && (APPLICATIONS_SORT_VALUES as readonly string[]).includes(raw)) {
      return raw as ApplicationsSort;
    }
  } catch {
    // localStorage may throw in private browsing — fall through to default
  }
  return DEFAULT_SORT;
}

function persistSort(sort: ApplicationsSort): void {
  try {
    window.localStorage.setItem(SORT_STORAGE_KEY, sort);
  } catch {
    // ignore — best-effort persistence
  }
}

export interface UseApplicationsOptions {
  readonly browsePort: ApplicationsBrowsePort;
  readonly zones: readonly WatchZoneSummary[];
  readonly notificationStateRepository: NotificationStateRepository;
}

interface State {
  readonly selectedZone: WatchZoneSummary | null;
  /** Accumulated rows across every page fetched so far for the current query. */
  readonly applications: readonly PlanningApplicationSummary[];
  /** True while the first page of a (re)query is in flight. */
  readonly isLoading: boolean;
  /** True while a subsequent page (load-more) is in flight. */
  readonly isLoadingMore: boolean;
  readonly error: string | null;
  readonly selectedStatusFilter: ApplicationStatus | null;
  readonly unreadOnly: boolean;
  readonly sort: ApplicationsSort;
  /** Cursor for the next page; `null` once the last page has been reached. */
  readonly nextCursor: string | null;
  /** Bumped to force a page-1 refetch without changing the query inputs (retry). */
  readonly reloadNonce: number;
}

function extractError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return 'Unknown error';
}

export function useApplications(options: UseApplicationsOptions) {
  const { browsePort, zones, notificationStateRepository } = options;
  const [state, setState] = useState<State>(() => ({
    selectedZone: null,
    applications: [],
    isLoading: false,
    isLoadingMore: false,
    error: null,
    selectedStatusFilter: null,
    unreadOnly: false,
    sort: readPersistedSort(),
    nextCursor: null,
    reloadNonce: 0,
  }));
  const hasAutoSelectedRef = useRef(false);

  // Monotonic generation counter. Each page-1 (re)query and each markAllRead
  // refetch bumps it; an in-flight load-more captures the current value and
  // discards its result if a newer query has since superseded it (e.g. the
  // user changed sort/filter mid-load). This is what makes "reset to page 1
  // on sort/filter change" race-safe.
  const requestIdRef = useRef(0);

  // Latest query inputs + pagination flags, mirrored into a ref so the
  // event-driven `loadMore`/`markAllRead` callbacks read fresh values without
  // re-subscribing. We build a fresh object (never the useState value) to stay
  // clear of the react-hooks immutability rule.
  const latestRef = useRef({
    zone: null as WatchZoneSummary | null,
    sort: state.sort,
    status: null as ApplicationStatus | null,
    unread: false,
    nextCursor: null as string | null,
    isLoading: false,
    isLoadingMore: false,
  });
  useEffect(() => {
    latestRef.current = {
      zone: state.selectedZone,
      sort: state.sort,
      status: state.selectedStatusFilter,
      unread: state.unreadOnly,
      nextCursor: state.nextCursor,
      isLoading: state.isLoading,
      isLoadingMore: state.isLoadingMore,
    };
  });

  // Auto-select the first zone the first time zones become non-empty. The query
  // effect below reacts to the resulting `selectedZone` change and fetches.
  useEffect(() => {
    if (hasAutoSelectedRef.current) return;
    if (zones.length === 0) return;
    hasAutoSelectedRef.current = true;
    const firstZone = zones[0]!;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setState((prev) => ({ ...prev, selectedZone: firstZone }));
  }, [zones]);

  // Page-1 query. Reacts to any change of the query inputs (zone, sort,
  // status, unread) or an explicit reload, always resetting pagination to the
  // first page (cursor null, accumulated rows discarded).
  useEffect(() => {
    if (state.selectedZone === null) return;
    const zone = state.selectedZone;
    const sort = state.sort;
    const status = state.selectedStatusFilter;
    const unread = state.unreadOnly;
    const requestId = ++requestIdRef.current;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setState((prev) => ({
      ...prev,
      applications: [],
      nextCursor: null,
      isLoading: true,
      isLoadingMore: false,
      error: null,
    }));
    browsePort
      .fetchByZone({ zoneId: zone.id, sort, status, unread, cursor: null })
      .then(({ rows, nextCursor }) => {
        if (requestId !== requestIdRef.current) return;
        setState((prev) => ({ ...prev, applications: rows, nextCursor, isLoading: false }));
      })
      .catch((err: unknown) => {
        if (requestId !== requestIdRef.current) return;
        setState((prev) => ({
          ...prev,
          applications: [],
          nextCursor: null,
          isLoading: false,
          error: extractError(err),
        }));
      });
  }, [
    state.selectedZone,
    state.sort,
    state.selectedStatusFilter,
    state.unreadOnly,
    state.reloadNonce,
    browsePort,
  ]);

  const selectZone = useCallback((zone: WatchZoneSummary) => {
    // Selecting a zone resets the status/unread filters (and, via the query
    // effect, pagination) to a clean first page.
    setState((prev) => ({
      ...prev,
      selectedZone: zone,
      selectedStatusFilter: null,
      unreadOnly: false,
    }));
  }, []);

  const setStatusFilter = useCallback((status: ApplicationStatus | null) => {
    // Status and Unread chips share a single-select group; selecting a status
    // clears unread-only so the two are never sent to the server together.
    setState((prev) => ({ ...prev, selectedStatusFilter: status, unreadOnly: false }));
  }, []);

  const setUnreadOnly = useCallback((on: boolean) => {
    setState((prev) => ({
      ...prev,
      unreadOnly: on,
      selectedStatusFilter: on ? null : prev.selectedStatusFilter,
    }));
  }, []);

  const setSort = useCallback((sort: ApplicationsSort) => {
    persistSort(sort);
    setState((prev) => ({ ...prev, sort }));
  }, []);

  const reload = useCallback(() => {
    setState((prev) => ({ ...prev, reloadNonce: prev.reloadNonce + 1 }));
  }, []);

  // Fetch the next page and append it. Guards against double-firing and against
  // running once the cursor is exhausted. A query that started before a newer
  // page-1 (re)query is dropped via the requestId check.
  const loadMore = useCallback(() => {
    const snap = latestRef.current;
    if (
      snap.zone === null ||
      snap.nextCursor === null ||
      snap.isLoading ||
      snap.isLoadingMore
    ) {
      return;
    }
    const requestId = requestIdRef.current;
    const zone = snap.zone;
    const cursor = snap.nextCursor;
    setState((prev) => ({ ...prev, isLoadingMore: true }));
    browsePort
      .fetchByZone({ zoneId: zone.id, sort: snap.sort, status: snap.status, unread: snap.unread, cursor })
      .then(({ rows, nextCursor }) => {
        if (requestId !== requestIdRef.current) return;
        setState((prev) => ({
          ...prev,
          applications: [...prev.applications, ...rows],
          nextCursor,
          isLoadingMore: false,
        }));
      })
      .catch((err: unknown) => {
        if (requestId !== requestIdRef.current) return;
        setState((prev) => ({ ...prev, isLoadingMore: false, error: extractError(err) }));
      });
  }, [browsePort]);

  const markAllRead = useCallback(async () => {
    // Server-side mark-all-read is idempotent. The page-1 refetch below replaces
    // every row with `latestUnreadEvent: null`, collapsing the derived
    // `unreadCount` to zero — no separate snapshot fetch needed (tc-u6bm).
    try {
      await notificationStateRepository.markAllRead();
    } catch {
      // Swallow — the post-mark refetch is the source of truth for unread state.
    }
    const snap = latestRef.current;
    if (snap.zone === null) return;
    const requestId = ++requestIdRef.current; // supersede any in-flight load-more
    try {
      const { rows, nextCursor } = await browsePort.fetchByZone({
        zoneId: snap.zone.id,
        sort: snap.sort,
        status: snap.status,
        unread: snap.unread,
        cursor: null,
      });
      if (requestId !== requestIdRef.current) return;
      setState((prev) => ({ ...prev, applications: rows, nextCursor }));
    } catch {
      // Refetch failure is non-fatal — the existing rows stay rendered.
    }
  }, [browsePort, notificationStateRepository]);

  // Derived: count of distinct loaded applications whose latest event is unread.
  // With server-side paging this reflects the rows fetched so far rather than
  // the whole zone — accurate for typical zones and for the unread-only view.
  const unreadCount = useMemo(
    () => state.applications.filter((app) => app.latestUnreadEvent !== null).length,
    [state.applications],
  );

  // Sort modes the picker should expose. `distance` is only meaningful relative
  // to a chosen zone, so it's hidden when no zone is active.
  const availableSortOptions = useMemo<readonly ApplicationsSort[]>(
    () =>
      APPLICATIONS_SORT_VALUES.filter(
        (mode) => mode !== 'distance' || state.selectedZone !== null,
      ),
    [state.selectedZone],
  );

  return {
    selectedZone: state.selectedZone,
    applications: state.applications,
    isLoading: state.isLoading,
    isLoadingMore: state.isLoadingMore,
    hasMore: state.nextCursor !== null,
    error: state.error,
    selectedStatusFilter: state.selectedStatusFilter,
    unreadOnly: state.unreadOnly,
    unreadCount,
    sort: state.sort,
    availableSortOptions,
    selectZone,
    setStatusFilter,
    setUnreadOnly,
    setSort,
    loadMore,
    reload,
    markAllRead,
  };
}
