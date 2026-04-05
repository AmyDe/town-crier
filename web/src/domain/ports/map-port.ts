import type { ApplicationUid, AuthorityId, AuthorityListItem, PlanningApplication, SavedApplication } from '../types';

export interface MapPort {
  fetchMyAuthorities(): Promise<readonly AuthorityListItem[]>;
  fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]>;
  fetchSavedApplications(): Promise<readonly SavedApplication[]>;
  saveApplication(uid: ApplicationUid): Promise<void>;
  unsaveApplication(uid: ApplicationUid): Promise<void>;
}
