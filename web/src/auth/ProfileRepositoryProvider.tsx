import type { ReactNode } from 'react';
import { useMemo } from 'react';
import { useApiClient } from '../api/useApiClient';
import { userProfileApi } from '../api/userProfile';
import { ProfileRepositoryProvider as ProfileRepoProvider } from './profile-context';
import type { ProfileRepository } from '../domain/ports/profile-repository';
import type { UserProfile } from '../domain/types';
import { ApiRequestError } from '../api/client';

interface Props {
  children: ReactNode;
}

export function ProfileRepositoryProvider({ children }: Props) {
  const client = useApiClient();

  const repository: ProfileRepository = useMemo(() => {
    const api = userProfileApi(client);
    return {
      async fetchProfile(): Promise<UserProfile | null> {
        try {
          return await api.get();
        } catch (err) {
          if (err instanceof ApiRequestError && err.status === 404) {
            return null;
          }
          throw err;
        }
      },
    };
  }, [client]);

  return (
    <ProfileRepoProvider value={repository}>
      {children}
    </ProfileRepoProvider>
  );
}
