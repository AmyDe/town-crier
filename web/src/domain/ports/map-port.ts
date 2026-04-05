import type { AuthorityId, AuthorityListItem, PlanningApplication } from '../types';

export interface MapPort {
  fetchMyAuthorities(): Promise<readonly AuthorityListItem[]>;
  fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]>;
}
