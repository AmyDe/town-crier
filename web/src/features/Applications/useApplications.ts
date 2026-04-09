import { useState, useCallback } from 'react';
import type { AuthorityListItem, PlanningApplicationSummary } from '../../domain/types';
import type { ApplicationsBrowsePort } from '../../domain/ports/applications-browse-port';
import { useFetchData } from '../../hooks/useFetchData';

export function useApplications(port: ApplicationsBrowsePort) {
  const [selectedAuthority, setSelectedAuthority] = useState<AuthorityListItem | null>(null);
  const [fetchKey, setFetchKey] = useState(0);

  const { data, isLoading, error } = useFetchData<readonly PlanningApplicationSummary[]>(
    () => port.fetchByAuthority(selectedAuthority!.id),
    [selectedAuthority?.id, fetchKey],
    { enabled: selectedAuthority !== null },
  );

  const selectAuthority = useCallback(
    (authority: AuthorityListItem | null) => {
      setSelectedAuthority(authority);
      setFetchKey((k) => k + 1);
    },
    [],
  );

  return {
    selectedAuthority,
    applications: data ?? [],
    isLoading,
    error,
    selectAuthority,
  };
}
