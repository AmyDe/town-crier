import type { GroupSummary } from '../../domain/types';
import type { GroupsRepository } from '../../domain/ports/groups-repository';
import { useFetchData } from '../../hooks/useFetchData';

export function useUserGroups(repository: GroupsRepository) {
  const { data, isLoading, error, refresh } = useFetchData<readonly GroupSummary[]>(
    () => repository.listGroups(),
    [repository],
  );

  return { groups: data ?? [], isLoading, error, refresh };
}
