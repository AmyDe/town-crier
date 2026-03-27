import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { ApplicationUid, SavedApplication } from '../../domain/types';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';

const QUERY_KEY = ['saved-applications'] as const;

export function useSavedApplications(repository: SavedApplicationRepository) {
  const queryClient = useQueryClient();

  const query = useQuery<readonly SavedApplication[]>({
    queryKey: QUERY_KEY,
    queryFn: () => repository.listSaved(),
  });

  const mutation = useMutation({
    mutationFn: (uid: ApplicationUid) => repository.remove(uid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  return {
    savedApplications: query.data ?? [],
    isLoading: query.isLoading,
    error: query.error instanceof Error ? query.error.message : null,
    remove: (uid: ApplicationUid) => mutation.mutate(uid),
    isRemoving: mutation.isPending,
  };
}
