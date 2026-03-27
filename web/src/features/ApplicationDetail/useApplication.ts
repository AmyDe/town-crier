import { useState, useEffect, useCallback } from 'react';
import type { ApplicationUid, PlanningApplication } from '../../domain/types';
import type { ApplicationRepository } from '../../domain/ports/application-repository';

interface ApplicationState {
  application: PlanningApplication | null;
  isLoading: boolean;
  error: string | null;
}

export function useApplication(repository: ApplicationRepository, uid: ApplicationUid) {
  const [state, setState] = useState<ApplicationState>({
    application: null,
    isLoading: true,
    error: null,
  });

  const fetchApplication = useCallback(async () => {
    setState({ application: null, isLoading: true, error: null });
    try {
      const application = await repository.fetchApplication(uid);
      setState({ application, isLoading: false, error: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      setState({ application: null, isLoading: false, error: message });
    }
  }, [repository, uid]);

  useEffect(() => {
    fetchApplication();
  }, [fetchApplication]);

  return state;
}
