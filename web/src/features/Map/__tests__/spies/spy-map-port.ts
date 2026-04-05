import type { AuthorityId, AuthorityListItem, PlanningApplication } from '../../../../domain/types';
import type { MapPort } from '../../../../domain/ports/map-port';

export class SpyMapPort implements MapPort {
  fetchMyAuthoritiesCalls = 0;
  fetchMyAuthoritiesResult: readonly AuthorityListItem[] = [];
  fetchMyAuthoritiesError: Error | null = null;

  async fetchMyAuthorities(): Promise<readonly AuthorityListItem[]> {
    this.fetchMyAuthoritiesCalls++;
    if (this.fetchMyAuthoritiesError) {
      throw this.fetchMyAuthoritiesError;
    }
    return this.fetchMyAuthoritiesResult;
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
