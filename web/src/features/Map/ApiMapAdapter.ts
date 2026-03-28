import type { ApiClient } from '../../api/client';
import type { AuthorityId, PlanningApplication, WatchZoneSummary } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { watchZonesApi } from '../../api/watchZones';
import { applicationsApi } from '../../api/applications';

export class ApiMapAdapter implements MapPort {
  private readonly zones: ReturnType<typeof watchZonesApi>;
  private readonly apps: ReturnType<typeof applicationsApi>;

  constructor(client: ApiClient) {
    this.zones = watchZonesApi(client);
    this.apps = applicationsApi(client);
  }

  async fetchWatchZones(): Promise<readonly WatchZoneSummary[]> {
    return this.zones.list();
  }

  async fetchApplicationsByAuthority(authorityId: AuthorityId): Promise<readonly PlanningApplication[]> {
    return this.apps.getByAuthority(authorityId as number);
  }
}
