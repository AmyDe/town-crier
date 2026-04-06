import type { UpdateProfileRequest, UserProfile } from '../types';

export interface SettingsRepository {
  fetchProfile(): Promise<UserProfile>;
  updateProfile(request: UpdateProfileRequest): Promise<UserProfile>;
  exportData(): Promise<Blob>;
  deleteAccount(): Promise<void>;
}
