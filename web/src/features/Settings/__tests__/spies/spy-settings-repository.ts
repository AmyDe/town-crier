import type { SettingsRepository } from '../../../../domain/ports/settings-repository';
import type { UserProfile } from '../../../../domain/types';
import { freeUserProfile } from '../fixtures/user-profile.fixtures';

export class SpySettingsRepository implements SettingsRepository {
  fetchProfileCalls = 0;
  fetchProfileResult: UserProfile = freeUserProfile();
  fetchProfileError: Error | null = null;

  async fetchProfile(): Promise<UserProfile> {
    this.fetchProfileCalls++;
    if (this.fetchProfileError) {
      throw this.fetchProfileError;
    }
    return this.fetchProfileResult;
  }

  exportDataCalls = 0;
  exportDataResult: Blob = new Blob(['{}'], { type: 'application/json' });
  exportDataError: Error | null = null;

  async exportData(): Promise<Blob> {
    this.exportDataCalls++;
    if (this.exportDataError) {
      throw this.exportDataError;
    }
    return this.exportDataResult;
  }

  deleteAccountCalls = 0;
  deleteAccountError: Error | null = null;

  async deleteAccount(): Promise<void> {
    this.deleteAccountCalls++;
    if (this.deleteAccountError) {
      throw this.deleteAccountError;
    }
  }
}
