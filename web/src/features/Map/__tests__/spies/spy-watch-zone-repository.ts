import type {
  WatchZoneSummary,
  CreateWatchZoneRequest,
  ZoneNotificationPreferences,
  UpdateZonePreferencesRequest,
} from '../../../../domain/types';
import type { WatchZoneRepository } from '../../../../domain/ports/watch-zone-repository';

export class SpyWatchZoneRepository implements WatchZoneRepository {
  listCalls = 0;
  listResult: readonly WatchZoneSummary[] = [];
  listError: Error | null = null;

  async list(): Promise<readonly WatchZoneSummary[]> {
    this.listCalls++;
    if (this.listError) {
      throw this.listError;
    }
    return this.listResult;
  }

  createCalls: CreateWatchZoneRequest[] = [];
  createResult: WatchZoneSummary | null = null;

  async create(data: CreateWatchZoneRequest): Promise<WatchZoneSummary> {
    this.createCalls.push(data);
    return this.createResult!;
  }

  deleteCalls: string[] = [];

  async delete(zoneId: string): Promise<void> {
    this.deleteCalls.push(zoneId);
  }

  getPreferencesCalls: string[] = [];
  getPreferencesResult: ZoneNotificationPreferences | null = null;

  async getPreferences(zoneId: string): Promise<ZoneNotificationPreferences> {
    this.getPreferencesCalls.push(zoneId);
    return this.getPreferencesResult!;
  }

  updatePreferencesCalls: Array<{ zoneId: string; data: UpdateZonePreferencesRequest }> = [];

  async updatePreferences(zoneId: string, data: UpdateZonePreferencesRequest): Promise<void> {
    this.updatePreferencesCalls.push({ zoneId, data });
  }
}
