import type { UserProfile } from '../../../domain/types.ts';
import type { ProfileRepository } from '../../../domain/ports/profile-repository.ts';

export class SpyProfileRepository implements ProfileRepository {
  fetchProfileCalls = 0;
  fetchProfileResult: UserProfile | null = null;
  fetchProfileError: Error | null = null;

  async fetchProfile(): Promise<UserProfile | null> {
    this.fetchProfileCalls += 1;
    if (this.fetchProfileError) {
      throw this.fetchProfileError;
    }
    return this.fetchProfileResult;
  }
}
