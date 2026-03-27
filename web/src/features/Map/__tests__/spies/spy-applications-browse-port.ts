import type { AuthorityId, PlanningApplication } from '../../../../domain/types';
import type { MapApplicationsPort } from '../../../../domain/ports/map-applications-port';

export class SpyMapApplicationsPort implements MapApplicationsPort {
  fetchByAuthorityCalls: AuthorityId[] = [];
  fetchByAuthorityResults: Map<number, readonly PlanningApplication[]> = new Map();
  fetchByAuthorityError: Error | null = null;

  async fetchByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]> {
    this.fetchByAuthorityCalls.push(authorityId);
    if (this.fetchByAuthorityError) {
      throw this.fetchByAuthorityError;
    }
    return this.fetchByAuthorityResults.get(authorityId as number) ?? [];
  }
}
