import type { AuthorityId, PlanningApplication, WatchZoneSummary } from '../types';

export interface MapPort {
  fetchWatchZones(): Promise<readonly WatchZoneSummary[]>;
  fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]>;
}
