import type { AuthorityId, PlanningApplicationSummary } from '../types';

export interface ApplicationsBrowsePort {
  fetchByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplicationSummary[]>;
}
