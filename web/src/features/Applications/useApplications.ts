import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import type {
  WatchZoneSummary,
  PlanningApplicationSummary,
  ApplicationStatus,
  NotificationStateSnapshot,
} from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { NotificationStateRepository } from '../../domain/ports/notification-state-repository';

/**
 * Sort modes for the Applications screen — persisted under
 * `applicationsListSort` in localStorage. Default: `recent-activity`.
 *
 * Spec: `docs/specs/notifications-unread-watermark.md` Pre-Resolved
 * Decisions #9 (default sort) and #10 (4 client-side options persisted).
 */
export type ApplicationsSort =
  | 'recent-activity'
  | 'newest'
  | 'oldest'
  | 'status';

const APPLICATIONS_SORT_VALUES: readonly ApplicationsSort[] = [
  'recent-activity',
  'newest',
  'oldest',
  'status',
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
  readonly error: string | null;
  readonly selectedStatusFilter: ApplicationStatus | null;
  readonly unreadOnly: boolean;
  readonly notificationState: NotificationStateSnapshot | null;
  readonly sort: ApplicationsSort;
}

function extractError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return 'Unknown error';
}

function recentActivityScore(app: PlanningApplicationSummary): number {
  const startDateMs = app.startDate ? Date.parse(app.startDate) : 0;
  const unreadMs = app.latestUnreadEvent
    ? Date.parse(app.latestUnreadEvent.createdAt)
    : 0;
  return Math.max(startDateMs, unreadMs);
}

function startDateMs(app: PlanningApplicationSummary): number {
  return app.startDate ? Date.parse(app.startDate) : 0;
}

function sortApplications(
  applications: readonly PlanningApplicationSummary[],
  sort: ApplicationsSort,
): readonly PlanningApplicationSummary[] {
  const copy = [...applications];
  switch (sort) {
    case 'recent-activity':
      copy.sort((a, b) => recentActivityScore(b) - recentActivityScore(a));
      return copy;
    case 'newest':
      copy.sort((a, b) => startDateMs(b) - startDateMs(a));
      return copy;
    case 'oldest':
      copy.sort((a, b) => startDateMs(a) - startDateMs(b));
      return copy;
    case 'status':
      copy.sort((a, b) => a.appState.localeCompare(b.appState));
      return copy;
  }
}

export function useApplications(options: UseApplicationsOptions) {
  const { browsePort, zones, notificationStateRepository } = options;
  const [state, setState] = useState<State>(() => ({
    selectedZone: null,
    applications: [],
    isLoading: false,
    error: null,
    selectedStatusFilter: null,
    unreadOnly: false,
    notificationState: null,
    sort: readPersistedSort(),
  }));
  const hasAutoSelectedRef = useRef(false);

  // Fetch the watermark snapshot once on mount. A failure is silent —
  // the chip shows zero rather than blocking the screen.
  useEffect(() => {
    let cancelled = false;
    notificationStateRepository
      .getState()
      .then((snapshot) => {
        if (cancelled) return;
        setState((prev) => ({ ...prev, notificationState: snapshot }));
      })
      .catch(() => {
        // Silent fallback per spec — the Unread chip just hides.
      });
    return () => {
      cancelled = true;
    };
  }, [notificationStateRepository]);

  // Auto-select the first zone the first time zones become non-empty.
  useEffect(() => {
    if (hasAutoSelectedRef.current) return;
    if (zones.length === 0) return;
    hasAutoSelectedRef.current = true;
    const firstZone = zones[0]!;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setState((prev) => ({
      ...prev,
      selectedZone: firstZone,
      isLoading: true,
      error: null,
    }));
    browsePort
      .fetchByZone(firstZone.id)
      .then((apps) =>
        setState((prev) => ({ ...prev, applications: apps, isLoading: false })),
      )
      .catch((err: unknown) =>
        setState((prev) => ({
          ...prev,
          applications: [],
          isLoading: false,
          error: extractError(err),
        })),
      );
  }, [zones, browsePort]);

  const selectZone = useCallback(
    (zone: WatchZoneSummary) => {
      setState((prev) => ({
        ...prev,
        selectedZone: zone,
        selectedStatusFilter: null,
        unreadOnly: false,
        isLoading: true,
        error: null,
      }));
      browsePort
        .fetchByZone(zone.id)
        .then((apps) =>
          setState((prev) => ({ ...prev, applications: apps, isLoading: false })),
        )
        .catch((err: unknown) =>
          setState((prev) => ({
            ...prev,
            applications: [],
            isLoading: false,
            error: extractError(err),
          })),
        );
    },
    [browsePort],
  );

  const setStatusFilter = useCallback((status: ApplicationStatus | null) => {
    // Status and Unread chips share a single-select group (spec decision #7);
    // selecting a status clears any unread-only mode.
    setState((prev) => ({
      ...prev,
      selectedStatusFilter: status,
      unreadOnly: false,
    }));
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

  // Track the currently-selected zone in a ref so async actions like
  // `markAllRead` can refetch the right zone after their await without
  // closing over stale state. We use a small ref tracking only the zone id
  // rather than mirroring the full state object — the latter conflicts with
  // the `react-hooks/immutability` rule that forbids mutating values handed
  // back from useState.
  const selectedZoneIdRef = useRef<WatchZoneSummary['id'] | null>(
    state.selectedZone?.id ?? null,
  );
  useEffect(() => {
    selectedZoneIdRef.current = state.selectedZone?.id ?? null;
  }, [state.selectedZone]);

  const markAllRead = useCallback(async () => {
    // Optimistic local state per spec decision #8 (silent optimistic). The
    // server treats mark-all-read as idempotent; we refresh the row data so
    // every `latestUnreadEvent` drops to null in one go.
    setState((prev) => ({
      ...prev,
      notificationState: prev.notificationState
        ? { ...prev.notificationState, totalUnreadCount: 0 }
        : prev.notificationState,
    }));
    try {
      await notificationStateRepository.markAllRead();
    } catch {
      // Swallow — the optimistic UI already shows the desired result. A
      // subsequent state fetch will correct any drift.
    }
    const activeZoneId = selectedZoneIdRef.current;
    if (activeZoneId !== null) {
      try {
        const apps = await browsePort.fetchByZone(activeZoneId);
        setState((prev) => ({ ...prev, applications: apps }));
      } catch {
        // Refetch failure is non-fatal — the existing rows stay rendered.
      }
    }
  }, [browsePort, notificationStateRepository]);

  // Derived: filtered, sorted applications.
  const filteredApplications = useMemo<readonly PlanningApplicationSummary[]>(() => {
    let rows: readonly PlanningApplicationSummary[] = state.applications;
    if (state.unreadOnly) {
      rows = rows.filter((app) => app.latestUnreadEvent !== null);
    } else if (state.selectedStatusFilter !== null) {
      rows = rows.filter((app) => app.appState === state.selectedStatusFilter);
    }
    return sortApplications(rows, state.sort);
  }, [
    state.applications,
    state.selectedStatusFilter,
    state.unreadOnly,
    state.sort,
  ]);

  return {
    selectedZone: state.selectedZone,
    applications: filteredApplications,
    isLoading: state.isLoading,
    error: state.error,
    selectedStatusFilter: state.selectedStatusFilter,
    unreadOnly: state.unreadOnly,
    unreadCount: state.notificationState?.totalUnreadCount ?? 0,
    notificationState: state.notificationState,
    sort: state.sort,
    selectZone,
    setStatusFilter,
    setUnreadOnly,
    setSort,
    markAllRead,
  };
}
