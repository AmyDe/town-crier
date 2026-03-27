import { useState, useEffect, useCallback } from 'react';
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

  const fetchDesignations = useCallback(async () => {
    if (latitude === null || longitude === null) {
      return;
    }
    setState({ designations: null, isLoading: true, error: null });
    try {
      const designations = await repository.fetchDesignations(latitude, longitude);
      setState({ designations, isLoading: false, error: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setState({ designations: null, isLoading: false, error: message });
    }
  }, [repository, latitude, longitude]);

  useEffect(() => {
    if (hasCoordinates) {
      fetchDesignations();
    }
  }, [hasCoordinates, fetchDesignations]);

  return state;
}
