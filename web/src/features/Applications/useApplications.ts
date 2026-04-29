import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import type {
  WatchZoneSummary,
  PlanningApplicationSummary,
  ApplicationStatus,
} from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';

export interface UseApplicationsOptions {
  readonly browsePort: ApplicationsBrowsePort;
  readonly zones: readonly WatchZoneSummary[];
}

interface State {
  readonly selectedZone: WatchZoneSummary | null;
  readonly applications: readonly PlanningApplicationSummary[];
  readonly isLoading: boolean;
  readonly error: string | null;
  readonly selectedStatusFilter: ApplicationStatus | null;
}

const INITIAL_STATE: State = {
  selectedZone: null,
  applications: [],
  isLoading: false,
  error: null,
  selectedStatusFilter: null,
};

function extractError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return 'Unknown error';
}

export function useApplications(options: UseApplicationsOptions) {
  const { browsePort, zones } = options;
  const [state, setState] = useState<State>(INITIAL_STATE);
  const hasAutoSelectedRef = useRef(false);

  // Auto-select the first zone the first time zones become non-empty.
  // setState here syncs UI to a one-shot async upstream load (zones from
  // the profile fetch); the ref guards against repeats.
  useEffect(() => {
    if (hasAutoSelectedRef.current) return;
    if (zones.length === 0) return;
    hasAutoSelectedRef.current = true;
    const firstZone = zones[0]!;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setState((prev) => ({ ...prev, selectedZone: firstZone, isLoading: true, error: null }));
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
    setState((prev) => ({ ...prev, selectedStatusFilter: status }));
  }, []);

  // Derived: filtered applications.
  const filteredApplications = useMemo<readonly PlanningApplicationSummary[]>(() => {
    if (state.selectedStatusFilter === null) return state.applications;
    return state.applications.filter((app) => app.appState === state.selectedStatusFilter);
  }, [state.applications, state.selectedStatusFilter]);

  return {
    selectedZone: state.selectedZone,
    applications: filteredApplications,
    isLoading: state.isLoading,
    error: state.error,
    selectedStatusFilter: state.selectedStatusFilter,
    selectZone,
    setStatusFilter,
  };
}
