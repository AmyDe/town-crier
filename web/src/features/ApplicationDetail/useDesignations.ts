import type { DesignationContext } from '../../domain/types';
import type { DesignationRepository } from '../../domain/ports/designation-repository';
import { useFetchData } from '../../hooks/useFetchData';

export function useDesignations(
  repository: DesignationRepository,
  latitude: number | null,
  longitude: number | null,
) {
  const hasCoordinates = latitude !== null && longitude !== null;

  const { data: designations, isLoading, error } = useFetchData<DesignationContext>(
    () => repository.fetchDesignations(latitude!, longitude!),
    [repository, latitude, longitude],
    { enabled: hasCoordinates },
  );

  return { designations, isLoading, error };
}
