import { useState, useEffect, useCallback, useMemo } from 'react';
import type {
  PlanningApplicationSummary,
  ApplicationStatus,
} from '../../domain/types';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';

export interface UseSavedApplicationsOptions {
  readonly savedRepository: SavedApplicationRepository;
}

interface State {
  readonly applications: readonly PlanningApplicationSummary[];
  readonly isLoading: boolean;
  readonly error: string | null;
  readonly selectedStatusFilter: ApplicationStatus | null;
}

const INITIAL_STATE: State = {
  applications: [],
  isLoading: true,
  error: null,
  selectedStatusFilter: null,
};

function extractError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return 'Unknown error';
}

export function useSavedApplications(options: UseSavedApplicationsOptions) {
  const { savedRepository } = options;
  const [state, setState] = useState<State>(INITIAL_STATE);

  useEffect(() => {
    let cancelled = false;
    savedRepository
      .listSaved()
      .then((saved) => {
        if (cancelled) return;
        const sorted = [...saved].sort(
          (a, b) => Date.parse(b.savedAt) - Date.parse(a.savedAt),
        );
        setState((prev) => ({
          ...prev,
          applications: sorted.map((s) => s.application),
          isLoading: false,
          error: null,
        }));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setState((prev) => ({
          ...prev,
          applications: [],
          isLoading: false,
          error: extractError(err),
        }));
      });
    return () => {
      cancelled = true;
    };
  }, [savedRepository]);

  const setStatusFilter = useCallback((status: ApplicationStatus | null) => {
    setState((prev) => ({ ...prev, selectedStatusFilter: status }));
  }, []);

  const filteredApplications = useMemo<readonly PlanningApplicationSummary[]>(() => {
    if (state.selectedStatusFilter === null) return state.applications;
    return state.applications.filter(
      (app) => app.appState === state.selectedStatusFilter,
    );
  }, [state.applications, state.selectedStatusFilter]);

  return {
    applications: filteredApplications,
    isLoading: state.isLoading,
    error: state.error,
    selectedStatusFilter: state.selectedStatusFilter,
    setStatusFilter,
  };
}
