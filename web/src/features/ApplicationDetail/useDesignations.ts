import type { DesignationContext } from '../../domain/types';
import type { DesignationRepository } from '../../domain/ports/designation-repository';
import { useFetchData } from '../../hooks/useFetchData';

export function useDesignations(
  repository: DesignationRepository,
  latitude: number | null,
  longitude: number | null,
) {
  const hasCoordinates = latitude !== null && longitude !== null;

  const fetchResult = useFetchData<DesignationContext>(
    () => repository.fetchDesignations(latitude!, longitude!),
    [repository, latitude, longitude],
  );

  if (!hasCoordinates) {
    return {
      designations: null as DesignationContext | null,
      isLoading: false,
      error: null as string | null,
    };
  }

  return {
    designations: fetchResult.data,
    isLoading: fetchResult.isLoading,
    error: fetchResult.error,
  };
}
