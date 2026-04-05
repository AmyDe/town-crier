import type { ApiClient } from '../../api/client';
import type { AuthorityId, AuthorityListItem, PlanningApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { applicationsApi } from '../../api/applications';

export class ApiMapAdapter implements MapPort {
  private readonly apps: ReturnType<typeof applicationsApi>;

  constructor(client: ApiClient) {
    this.apps = applicationsApi(client);
  }

  async fetchMyAuthorities(): Promise<readonly AuthorityListItem[]> {
    return this.apps.getMyAuthorities();
  }

  async fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]> {
    return this.apps.getByAuthority(authorityId as number);
  }
}
