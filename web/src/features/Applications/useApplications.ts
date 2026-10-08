import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import type {
  WatchZoneSummary,
  PlanningApplicationSummary,
  ApplicationStatus,
  ApplicationsSort,
} from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { NotificationStateRepository } from '../../domain/ports/notification-state-repository';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

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
  readonly applications: readonly PlanningApplicationSummary[];
  readonly isLoading: boolean;
  readonly isLoadingMore: boolean;
  readonly error: string | null;
  readonly selectedStatusFilter: ApplicationStatus | null;
  readonly unreadOnly: boolean;
  readonly sort: ApplicationsSort;
  readonly nextCursor: string | null;
  /** Bumped to force a page-1 refetch without changing the query inputs (retry). */
  readonly reloadNonce: number;
  /** Whole-zone unread total from `browsePort.countUnread`, not derived from the loaded rows. */
  readonly unreadCount: number;
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

  // Bumped by every page-1 query and markAllRead; a load-more that started
  // under an older value drops its result.
  const requestIdRef = useRef(0);

  // Separate from requestIdRef so a count refresh never supersedes an in-flight list page.
  const countUnreadRequestIdRef = useRef(0);

  // A fresh object rather than the useState value, to stay clear of the
  // react-hooks immutability rule.
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

  useEffect(() => {
    if (hasAutoSelectedRef.current) return;
    if (zones.length === 0) return;
    hasAutoSelectedRef.current = true;
    const firstZone = zones[0]!;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setState((prev) => ({ ...prev, selectedZone: firstZone }));
  }, [zones]);

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
          error: extractErrorMessage(err, 'Unknown error'),
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

  // Refetched only on zone change so filters and paging never move the chip's
  // number; markAllRead refreshes it explicitly.
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
    setState((prev) => ({
      ...prev,
      selectedZone: zone,
      selectedStatusFilter: null,
      unreadOnly: false,
    }));
  }, []);

  const setStatusFilter = useCallback((status: ApplicationStatus | null) => {
    // The server rejects status and unread sent together.
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
        setState((prev) => ({ ...prev, isLoadingMore: false, error: extractErrorMessage(err, 'Unknown error') }));
      });
  }, [browsePort]);

  const markAllRead = useCallback(async () => {
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

  const onOpenApplication = useCallback(
    (application: PlanningApplicationSummary) => {
      if (application.latestUnreadEvent === null) return;
      // The wire field `applicationUid` carries the application's name, not its
      // uid; `areaId` disambiguates same-name refs across councils.
      void notificationStateRepository
        .markApplicationRead(application.name, application.areaId)
        .catch(() => {
          // Intentionally ignored — the next fetch is the source of truth.
        });
      // Clamped so repeat opens can't drive the count negative.
      setState((prev) => ({
        ...prev,
        unreadCount: Math.max(0, prev.unreadCount - 1),
      }));
    },
    [notificationStateRepository],
  );

  // `distance` is only meaningful relative to a chosen zone.
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
    unreadCount: state.unreadCount,
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
