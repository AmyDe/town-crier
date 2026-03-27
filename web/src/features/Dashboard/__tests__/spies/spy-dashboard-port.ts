import type { DashboardPort } from '../../../../domain/ports/dashboard-port';
import type { AuthorityId, PlanningApplicationSummary, WatchZoneSummary } from '../../../../domain/types';

export class SpyDashboardPort implements DashboardPort {
  fetchWatchZonesCalls = 0;
  fetchWatchZonesResult: readonly WatchZoneSummary[] = [];

  async fetchWatchZones(): Promise<readonly WatchZoneSummary[]> {
    this.fetchWatchZonesCalls += 1;
    return this.fetchWatchZonesResult;
  }

  fetchRecentApplicationsCalls: AuthorityId[] = [];
  fetchRecentApplicationsResults: Map<AuthorityId, readonly PlanningApplicationSummary[]> = new Map();

  async fetchRecentApplications(authorityId: AuthorityId): Promise<readonly PlanningApplicationSummary[]> {
    this.fetchRecentApplicationsCalls.push(authorityId);
    return this.fetchRecentApplicationsResults.get(authorityId) ?? [];
  }
}
