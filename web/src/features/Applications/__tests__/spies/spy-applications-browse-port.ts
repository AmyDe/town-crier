import type { WatchZoneId, PlanningApplicationSummary } from '../../../../domain/types';
import type { ApplicationsBrowsePort } from '../../../../domain/ports/applications-browse-port';

export class SpyApplicationsBrowsePort implements ApplicationsBrowsePort {
  fetchByZoneCalls: WatchZoneId[] = [];
  fetchByZoneResult: readonly PlanningApplicationSummary[] = [];
  fetchByZoneError: Error | null = null;
  fetchByZoneOverride: ((zoneId: WatchZoneId) => Promise<readonly PlanningApplicationSummary[]>) | null = null;

  async fetchByZone(zoneId: WatchZoneId): Promise<readonly PlanningApplicationSummary[]> {
    this.fetchByZoneCalls.push(zoneId);
    if (this.fetchByZoneOverride) {
      return this.fetchByZoneOverride(zoneId);
    }
    if (this.fetchByZoneError) {
      throw this.fetchByZoneError;
    }
    return this.fetchByZoneResult;
  }
}
