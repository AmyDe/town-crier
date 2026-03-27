import type { UserProfile } from '../types';

export interface SettingsRepository {
  fetchProfile(): Promise<UserProfile>;
  exportData(): Promise<Blob>;
  deleteAccount(): Promise<void>;
}
