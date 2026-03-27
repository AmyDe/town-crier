import { useState, useCallback } from 'react';
import type { AuthorityListItem, PlanningApplicationSummary } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';

interface ApplicationsState {
  selectedAuthority: AuthorityListItem | null;
  applications: readonly PlanningApplicationSummary[];
  isLoading: boolean;
  error: Error | null;
}

export function useApplications(port: ApplicationsBrowsePort) {
  const [state, setState] = useState<ApplicationsState>({
    selectedAuthority: null,
    applications: [],
    isLoading: false,
    error: null,
  });

  const selectAuthority = useCallback(
    (authority: AuthorityListItem) => {
      setState((prev) => ({
        ...prev,
        selectedAuthority: authority,
        isLoading: true,
        error: null,
      }));

      port
        .fetchByAuthority(authority.id)
        .then((applications) => {
          setState((prev) => ({
            ...prev,
            applications,
            isLoading: false,
          }));
        })
        .catch((err: unknown) => {
          setState((prev) => ({
            ...prev,
            applications: [],
            isLoading: false,
            error: err instanceof Error ? err : new Error(String(err)),
          }));
        });
    },
    [port],
  );

  return {
    selectedAuthority: state.selectedAuthority,
    applications: state.applications,
    isLoading: state.isLoading,
    error: state.error,
    selectAuthority,
  };
}
