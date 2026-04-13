import type { DashboardPort } from '../../../../domain/ports/dashboard-port';
import type { WatchZoneId, PlanningApplicationSummary, WatchZoneSummary } from '../../../../domain/types';

export class SpyDashboardPort implements DashboardPort {
  fetchWatchZonesCalls = 0;
  fetchWatchZonesResult: readonly WatchZoneSummary[] = [];

  async fetchWatchZones(): Promise<readonly WatchZoneSummary[]> {
    this.fetchWatchZonesCalls += 1;
    return this.fetchWatchZonesResult;
  }

  fetchRecentApplicationsCalls: WatchZoneId[] = [];
  fetchRecentApplicationsResults: Map<string, readonly PlanningApplicationSummary[]> = new Map();

  async fetchRecentApplications(zoneId: WatchZoneId): Promise<readonly PlanningApplicationSummary[]> {
    this.fetchRecentApplicationsCalls.push(zoneId);
    return this.fetchRecentApplicationsResults.get(zoneId as string) ?? [];
  }
}
