import type { AuthorityId, PlanningApplication, WatchZoneSummary } from '../../../../domain/types';
import type { MapPort } from '../../../../domain/ports/map-port';

export class SpyMapPort implements MapPort {
  fetchWatchZonesCalls = 0;
  fetchWatchZonesResult: readonly WatchZoneSummary[] = [];
  fetchWatchZonesError: Error | null = null;

  async fetchWatchZones(): Promise<readonly WatchZoneSummary[]> {
    this.fetchWatchZonesCalls++;
    if (this.fetchWatchZonesError) {
      throw this.fetchWatchZonesError;
    }
    return this.fetchWatchZonesResult;
  }

  fetchApplicationsByAuthorityCalls: AuthorityId[] = [];
  fetchApplicationsByAuthorityResults: Map<number, readonly PlanningApplication[]> = new Map();
  fetchApplicationsByAuthorityError: Error | null = null;

  async fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]> {
    this.fetchApplicationsByAuthorityCalls.push(authorityId);
    if (this.fetchApplicationsByAuthorityError) {
      throw this.fetchApplicationsByAuthorityError;
    }
    return this.fetchApplicationsByAuthorityResults.get(authorityId as number) ?? [];
  }
}
