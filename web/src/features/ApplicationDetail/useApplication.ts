import type { ApplicationRepository } from '../../domain/ports/application-repository';
import { useFetchData } from '../../hooks/useFetchData';

export function useApplication(
  repository: ApplicationRepository,
  authority: string | null,
  name: string | null,
) {
  const enabled = authority !== null && name !== null;
  const { data: application, isLoading, error } = useFetchData(
    () => repository.fetchApplication(authority!, name!),
    [repository, authority, name],
    { enabled },
  );

  return { application, isLoading, error };
}
