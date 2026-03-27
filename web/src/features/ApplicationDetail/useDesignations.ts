import { useState, useEffect } from 'react';
import type { DesignationContext } from '../../domain/types';
import type { DesignationRepository } from '../../domain/ports/designation-repository';

interface DesignationsState {
  designations: DesignationContext | null;
  isLoading: boolean;
  error: string | null;
}

export function useDesignations(
  repository: DesignationRepository,
  latitude: number | null,
  longitude: number | null,
) {
  const hasCoordinates = latitude !== null && longitude !== null;

  const [state, setState] = useState<DesignationsState>({
    designations: null,
    isLoading: hasCoordinates,
    error: null,
  });

  useEffect(() => {
    if (!hasCoordinates) return;
    let cancelled = false;
    repository.fetchDesignations(latitude!, longitude!).then(designations => {
      if (!cancelled) {
        setState({ designations, isLoading: false, error: null });
      }
    }).catch((err: unknown) => {
      if (!cancelled) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        setState({ designations: null, isLoading: false, error: message });
      }
    });
    return () => { cancelled = true; };
  }, [hasCoordinates, repository, latitude, longitude]);

  return state;
}
