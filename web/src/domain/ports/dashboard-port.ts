import type { AuthorityId, PlanningApplicationSummary, WatchZoneSummary } from '../types';

export interface DashboardPort {
  fetchWatchZones(): Promise<readonly WatchZoneSummary[]>;
  fetchRecentApplications(authorityId: AuthorityId): Promise<readonly PlanningApplicationSummary[]>;
}
