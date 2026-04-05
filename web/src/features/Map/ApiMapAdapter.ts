import type { ApiClient } from '../../api/client';
import type { ApplicationUid, AuthorityId, AuthorityListItem, PlanningApplication, SavedApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { applicationsApi } from '../../api/applications';
import { savedApplicationsApi } from '../../api/savedApplications';

export class ApiMapAdapter implements MapPort {
  private readonly apps: ReturnType<typeof applicationsApi>;
  private readonly saved: ReturnType<typeof savedApplicationsApi>;

  constructor(client: ApiClient) {
    this.apps = applicationsApi(client);
    this.saved = savedApplicationsApi(client);
  }

  async fetchMyAuthorities(): Promise<readonly AuthorityListItem[]> {
    return this.apps.getMyAuthorities();
  }

  async fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]> {
    return this.apps.getByAuthority(authorityId as number);
  }

  async fetchSavedApplications(): Promise<readonly SavedApplication[]> {
    return this.saved.list();
  }

  async saveApplication(uid: ApplicationUid): Promise<void> {
    await this.saved.save(uid as string);
  }

  async unsaveApplication(uid: ApplicationUid): Promise<void> {
    await this.saved.remove(uid as string);
  }
}
