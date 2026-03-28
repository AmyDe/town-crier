import type { ProfileRepository } from '../domain/ports/profile-repository.ts';
import { createRequiredContext } from '../utils/createRequiredContext.ts';

export const [ProfileRepositoryProvider, useProfileRepository] = createRequiredContext<ProfileRepository>('ProfileRepositoryContext');
