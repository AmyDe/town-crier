import type { WatchZoneId, PlanningApplicationSummary } from '../types';

export interface ApplicationsBrowsePort {
  fetchByZone(zoneId: WatchZoneId): Promise<readonly PlanningApplicationSummary[]>;
}
