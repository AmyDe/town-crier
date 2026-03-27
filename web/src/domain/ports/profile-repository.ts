import type { UserProfile } from '../types.ts';

export interface ProfileRepository {
  fetchProfile(): Promise<UserProfile | null>;
}
