import type { WatchZoneId, PlanningApplicationSummary, WatchZoneSummary } from '../types';

export interface DashboardPort {
  fetchWatchZones(): Promise<readonly WatchZoneSummary[]>;
  fetchRecentApplications(zoneId: WatchZoneId): Promise<readonly PlanningApplicationSummary[]>;
}
