import { createContext, useContext } from 'react';
import type { ProfileRepository } from '../domain/ports/profile-repository.ts';

export const ProfileRepositoryContext = createContext<ProfileRepository | null>(null);

export function useProfileRepository(): ProfileRepository {
  const ctx = useContext(ProfileRepositoryContext);
  if (ctx === null) {
    throw new Error('useProfileRepository must be used within a ProfileRepositoryContext.Provider');
  }
  return ctx;
}
