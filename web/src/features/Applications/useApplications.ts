import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import type {
  WatchZoneSummary,
  PlanningApplicationSummary,
  ApplicationStatus,
  ApplicationUid,
} from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';

export interface UseApplicationsOptions {
  readonly browsePort: ApplicationsBrowsePort;
  readonly savedRepository: SavedApplicationRepository;
  readonly zones: readonly WatchZoneSummary[];
}

interface State {
  readonly selectedZone: WatchZoneSummary | null;
  readonly isAllZonesSelected: boolean;
  readonly applications: readonly PlanningApplicationSummary[];
  readonly isLoading: boolean;
  readonly error: string | null;
  readonly selectedStatusFilter: ApplicationStatus | null;
  readonly isSavedFilterActive: boolean;
  readonly savedUids: ReadonlySet<ApplicationUid>;
}

const INITIAL_STATE: State = {
  selectedZone: null,
  isAllZonesSelected: false,
  applications: [],
  isLoading: false,
  error: null,
  selectedStatusFilter: null,
  isSavedFilterActive: false,
  savedUids: new Set(),
};

function extractError(err: unknown): string {
  if (err instanceof Error) return err.message;
  return 'Unknown error';
}

export function useApplications(options: UseApplicationsOptions) {
  const { browsePort, savedRepository, zones } = options;
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
        isAllZonesSelected: false,
        isSavedFilterActive: false,
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

  const selectAllZones = useCallback(() => {
    setState((prev) => ({
      ...prev,
      isAllZonesSelected: true,
      selectedZone: null,
      isSavedFilterActive: false,
      selectedStatusFilter: null,
      applications: [],
      isLoading: false,
      error: null,
    }));
  }, []);

  const activateSavedFilter = useCallback(async () => {
    let saved: readonly { applicationUid: ApplicationUid; application: PlanningApplicationSummary }[] = [];
    try {
      saved = await savedRepository.listSaved();
    } catch {
      saved = [];
    }
    const uids = new Set(saved.map((s) => s.applicationUid));
    setState((prev) => {
      const next: State = {
        ...prev,
        isSavedFilterActive: true,
        selectedStatusFilter: null,
        savedUids: uids,
      };
      if (prev.isAllZonesSelected) {
        // 'All' + Saved → populate from saved payloads (server-denormalised).
        return { ...next, applications: saved.map((s) => s.application) };
      }
      return next;
    });
  }, [savedRepository]);

  const deactivateSavedFilter = useCallback(() => {
    setState((prev) => {
      if (prev.isAllZonesSelected) {
        // 'All' has no per-zone source — clear so the empty-state hint takes over.
        return { ...prev, isSavedFilterActive: false, applications: [], savedUids: new Set() };
      }
      return { ...prev, isSavedFilterActive: false, savedUids: new Set() };
    });
  }, []);

  const setStatusFilter = useCallback((status: ApplicationStatus | null) => {
    setState((prev) => ({
      ...prev,
      selectedStatusFilter: status,
      // Selecting a status clears Saved (mutually exclusive).
      isSavedFilterActive: status !== null ? false : prev.isSavedFilterActive,
      savedUids: status !== null ? new Set() : prev.savedUids,
    }));
  }, []);

  // Derived: filtered applications.
  const filteredApplications = useMemo<readonly PlanningApplicationSummary[]>(() => {
    if (state.isSavedFilterActive && !state.isAllZonesSelected) {
      // Real zone + Saved: intersect zone applications with saved UIDs.
      return state.applications.filter((app) => state.savedUids.has(app.uid));
    }
    if (state.selectedStatusFilter !== null) {
      return state.applications.filter((app) => app.appState === state.selectedStatusFilter);
    }
    return state.applications;
  }, [state.applications, state.isAllZonesSelected, state.isSavedFilterActive, state.savedUids, state.selectedStatusFilter]);

  return {
    selectedZone: state.selectedZone,
    isAllZonesSelected: state.isAllZonesSelected,
    applications: filteredApplications,
    isLoading: state.isLoading,
    error: state.error,
    selectedStatusFilter: state.selectedStatusFilter,
    isSavedFilterActive: state.isSavedFilterActive,
    selectZone,
    selectAllZones,
    activateSavedFilter,
    deactivateSavedFilter,
    setStatusFilter,
  };
}
