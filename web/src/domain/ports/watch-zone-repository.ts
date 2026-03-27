import type {
  WatchZoneSummary,
  CreateWatchZoneRequest,
  ZoneNotificationPreferences,
  UpdateZonePreferencesRequest,
} from '../types';

export interface WatchZoneRepository {
  list(): Promise<readonly WatchZoneSummary[]>;
  create(data: CreateWatchZoneRequest): Promise<WatchZoneSummary>;
  delete(zoneId: string): Promise<void>;
  getPreferences(zoneId: string): Promise<ZoneNotificationPreferences>;
  updatePreferences(zoneId: string, data: UpdateZonePreferencesRequest): Promise<void>;
}
