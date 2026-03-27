import type { AuthorityId, PlanningApplication } from '../types';

export interface MapApplicationsPort {
  fetchByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]>;
}
