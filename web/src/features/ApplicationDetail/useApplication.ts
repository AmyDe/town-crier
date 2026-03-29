import type { ApplicationUid } from '../../domain/types';
import type { ApplicationRepository } from '../../domain/ports/application-repository';
import { useFetchData } from '../../hooks/useFetchData';

export function useApplication(repository: ApplicationRepository, uid: ApplicationUid) {
  const { data: application, isLoading, error } = useFetchData(
    () => repository.fetchApplication(uid),
    [repository, uid],
  );

  return { application, isLoading, error };
}
