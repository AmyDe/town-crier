import type { ApiClient } from '../../api/client';
import type { SettingsRepository } from '../../domain/ports/settings-repository';
import type { UpdateProfileRequest, UserProfile } from '../../domain/types';
import { userProfileApi } from '../../api/userProfile';

export class ApiSettingsRepository implements SettingsRepository {
  private readonly api: ReturnType<typeof userProfileApi>;
  private readonly baseUrl: string;
  private readonly getToken: () => Promise<string>;

  constructor(
    client: ApiClient,
    baseUrl: string,
    getToken: () => Promise<string>,
  ) {
    this.api = userProfileApi(client);
    this.baseUrl = baseUrl;
    this.getToken = getToken;
  }

  async fetchProfile(): Promise<UserProfile> {
    return this.api.get();
  }

  async updateProfile(request: UpdateProfileRequest): Promise<UserProfile> {
    return this.api.update(request);
  }

  async exportData(): Promise<Blob> {
    const token = await this.getToken();
    const response = await fetch(`${this.baseUrl}/v1/me/data`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!response.ok) {
      throw new Error('Failed to export data');
    }
    return response.blob();
  }

  async deleteAccount(): Promise<void> {
    await this.api.delete();
  }
}
