import { useState, useEffect } from 'react';
import type { AuthorityListItem } from '../../domain/types';
import type { UserAuthoritiesPort } from '../../domain/ports/user-authorities-port';

interface UserAuthoritiesState {
  authorities: readonly AuthorityListItem[];
  isLoading: boolean;
  error: Error | null;
}

export function useUserAuthorities(port: UserAuthoritiesPort) {
  const [state, setState] = useState<UserAuthoritiesState>({
    authorities: [],
    isLoading: true,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;

    port
      .fetchMyAuthorities()
      .then((authorities) => {
        if (!cancelled) {
          setState({ authorities, isLoading: false, error: null });
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState({
            authorities: [],
            isLoading: false,
            error: err instanceof Error ? err : new Error(String(err)),
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [port]);

  return state;
}
