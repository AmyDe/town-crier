import type { AuthorityId, PlanningApplicationSummary } from '../../../../domain/types';
import type { ApplicationsBrowsePort } from '../../../../domain/ports/applications-browse-port';

export class SpyApplicationsBrowsePort implements ApplicationsBrowsePort {
  fetchByAuthorityCalls: AuthorityId[] = [];
  fetchByAuthorityResult: readonly PlanningApplicationSummary[] = [];
  fetchByAuthorityError: Error | null = null;
  fetchByAuthorityOverride: ((authorityId: AuthorityId) => Promise<readonly PlanningApplicationSummary[]>) | null = null;

  async fetchByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplicationSummary[]> {
    this.fetchByAuthorityCalls.push(authorityId);
    if (this.fetchByAuthorityOverride) {
      return this.fetchByAuthorityOverride(authorityId);
    }
    if (this.fetchByAuthorityError) {
      throw this.fetchByAuthorityError;
    }
    return this.fetchByAuthorityResult;
  }
}
