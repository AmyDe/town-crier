import type { ApplicationUid, AuthorityId, AuthorityListItem, PlanningApplication, SavedApplication } from '../../../../domain/types';
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

  fetchSavedApplicationsCalls = 0;
  fetchSavedApplicationsResult: readonly SavedApplication[] = [];
  fetchSavedApplicationsError: Error | null = null;

  async fetchSavedApplications(): Promise<readonly SavedApplication[]> {
    this.fetchSavedApplicationsCalls++;
    if (this.fetchSavedApplicationsError) {
      throw this.fetchSavedApplicationsError;
    }
    return this.fetchSavedApplicationsResult;
  }

  saveApplicationCalls: ApplicationUid[] = [];
  saveApplicationError: Error | null = null;

  async saveApplication(uid: ApplicationUid): Promise<void> {
    this.saveApplicationCalls.push(uid);
    if (this.saveApplicationError) {
      throw this.saveApplicationError;
    }
  }

  unsaveApplicationCalls: ApplicationUid[] = [];
  unsaveApplicationError: Error | null = null;

  async unsaveApplication(uid: ApplicationUid): Promise<void> {
    this.unsaveApplicationCalls.push(uid);
    if (this.unsaveApplicationError) {
      throw this.unsaveApplicationError;
    }
  }
}
