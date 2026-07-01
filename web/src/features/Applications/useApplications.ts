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
  /**
   * Whole-zone unread total for the "Unread (N)" chip (GH#716, Problem 2).
   * Sourced from `browsePort.countUnread(zone.id)` — independent of how many
   * main-list pages are loaded — not derived from the rows fetched so far.
   */
  readonly unreadCount: number;
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
    unreadCount: 0,
  }));
  const hasAutoSelectedRef = useRef(false);

  // Monotonic generation counter. Each page-1 (re)query and each markAllRead
  // refetch bumps it; an in-flight load-more captures the current value and
  // discards its result if a newer query has since superseded it (e.g. the
  // user changed sort/filter mid-load). This is what makes "reset to page 1
  // on sort/filter change" race-safe.
  const requestIdRef = useRef(0);

  // Separate generation counter for the whole-zone unread count. Kept distinct
  // from `requestIdRef` so a count refresh never supersedes an in-flight list
  // page (and vice versa); it only guards the count's own zone-change/markAllRead
  // refetches against stale responses overwriting a newer zone's total.
  const countUnreadRequestIdRef = useRef(0);

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

  // Whole-zone unread total for the chip. Re-fetched only when the selected zone
  // changes — deliberately independent of sort/status/unread/pagination, so
  // loading more list pages (or toggling filters) never moves the chip's number.
  // markAllRead refreshes it explicitly.
  useEffect(() => {
    if (state.selectedZone === null) return;
    const zoneId = state.selectedZone.id;
    const requestId = ++countUnreadRequestIdRef.current;
    browsePort
      .countUnread(zoneId)
      .then((count) => {
        if (requestId !== countUnreadRequestIdRef.current) return;
        setState((prev) => ({ ...prev, unreadCount: count }));
      })
      .catch(() => {
        // Non-fatal: leave the previous count in place rather than zeroing the
        // chip on a transient failure.
      });
  }, [state.selectedZone, browsePort]);

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
    // Server-side mark-all-read is idempotent. We then refresh two things in
    // parallel: page 1 (its rows now carry `latestUnreadEvent: null`, so the
    // cards render as read) and the whole-zone unread count (the server
    // watermark advanced, so it should drop — typically to zero).
    try {
      await notificationStateRepository.markAllRead();
    } catch {
      // Swallow — the post-mark refetch is the source of truth for unread state.
    }
    const snap = latestRef.current;
    if (snap.zone === null) return;
    const zoneId = snap.zone.id;
    const requestId = ++requestIdRef.current; // supersede any in-flight load-more
    const countRequestId = ++countUnreadRequestIdRef.current;
    await Promise.all([
      browsePort
        .fetchByZone({
          zoneId,
          sort: snap.sort,
          status: snap.status,
          unread: snap.unread,
          cursor: null,
        })
        .then(({ rows, nextCursor }) => {
          if (requestId !== requestIdRef.current) return;
          setState((prev) => ({ ...prev, applications: rows, nextCursor }));
        })
        .catch(() => {
          // Refetch failure is non-fatal — the existing rows stay rendered.
        }),
      browsePort
        .countUnread(zoneId)
        .then((count) => {
          if (countRequestId !== countUnreadRequestIdRef.current) return;
          setState((prev) => ({ ...prev, unreadCount: count }));
        })
        .catch(() => {
          // Non-fatal — leave the previous count in place.
        }),
    ]);
  }, [browsePort, notificationStateRepository]);

  // Tap-to-read: opening an application marks its notifications read server-side
  // (ADR 0035). Fired from the card's onClick; navigation proceeds regardless.
  const onOpenApplication = useCallback(
    (application: PlanningApplicationSummary) => {
      // Guardrail: only round-trip for a genuinely-unread card — an already-read
      // application (no unread event) needs no mark-read call.
      if (application.latestUnreadEvent === null) return;
      // Fire-and-forget: a later list/count fetch reconciles read state, so a
      // failure here is swallowed rather than surfaced on a navigation. The
      // wire field `applicationUid` carries the app's NAME (PlanIt case
      // reference), NOT its uid; `areaId` disambiguates same-name refs across
      // councils — see the notification-state API contract note.
      void notificationStateRepository
        .markApplicationRead(application.name, application.areaId)
        .catch(() => {
          // Intentionally ignored — the next fetch is the source of truth.
        });
      // Optimistically drop the whole-zone "Unread (N)" chip so it updates
      // immediately without a full count refetch (mirrors markAllRead's count
      // refresh). Clamp at zero so repeat opens can't drive it negative.
      setState((prev) => ({
        ...prev,
        unreadCount: Math.max(0, prev.unreadCount - 1),
      }));
    },
    [notificationStateRepository],
  );

  // Whole-zone unread total for the chip, sourced from `browsePort.countUnread`
  // (see the count-fetch effect above) rather than the loaded rows — so it
  // reflects the zone, not how far the user has paged (GH#716, Problem 2).
  const unreadCount = state.unreadCount;

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
    onOpenApplication,
  };
}
