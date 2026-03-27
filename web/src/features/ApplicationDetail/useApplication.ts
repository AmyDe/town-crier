import { useState, useEffect } from 'react';
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

  useEffect(() => {
    let cancelled = false;
    repository.fetchApplication(uid).then(application => {
      if (!cancelled) {
        setState({ application, isLoading: false, error: null });
      }
    }).catch((err: unknown) => {
      if (!cancelled) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        setState({ application: null, isLoading: false, error: message });
      }
    });
    return () => { cancelled = true; };
  }, [repository, uid]);

  return state;
}
