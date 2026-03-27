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
  createError: Error | null = null;

  async create(data: CreateWatchZoneRequest): Promise<WatchZoneSummary> {
    this.createCalls.push(data);
    if (this.createError) {
      throw this.createError;
    }
    return this.createResult!;
  }

  deleteCalls: string[] = [];
  deleteError: Error | null = null;

  async delete(zoneId: string): Promise<void> {
    this.deleteCalls.push(zoneId);
    if (this.deleteError) {
      throw this.deleteError;
    }
  }

  getPreferencesCalls: string[] = [];
  getPreferencesResult: ZoneNotificationPreferences | null = null;
  getPreferencesError: Error | null = null;

  async getPreferences(zoneId: string): Promise<ZoneNotificationPreferences> {
    this.getPreferencesCalls.push(zoneId);
    if (this.getPreferencesError) {
      throw this.getPreferencesError;
    }
    return this.getPreferencesResult!;
  }

  updatePreferencesCalls: Array<{ zoneId: string; data: UpdateZonePreferencesRequest }> = [];
  updatePreferencesError: Error | null = null;

  async updatePreferences(zoneId: string, data: UpdateZonePreferencesRequest): Promise<void> {
    this.updatePreferencesCalls.push({ zoneId, data });
    if (this.updatePreferencesError) {
      throw this.updatePreferencesError;
    }
  }
}
