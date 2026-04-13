import type { ApiClient } from '../../api/client';
import type { ApplicationUid, WatchZoneId, WatchZoneSummary, PlanningApplication, SavedApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { applicationsApi } from '../../api/applications';
import { watchZonesApi } from '../../api/watchZones';
import { savedApplicationsApi } from '../../api/savedApplications';

export class ApiMapAdapter implements MapPort {
  private readonly apps: ReturnType<typeof applicationsApi>;
  private readonly zones: ReturnType<typeof watchZonesApi>;
  private readonly saved: ReturnType<typeof savedApplicationsApi>;

  constructor(client: ApiClient) {
    this.apps = applicationsApi(client);
    this.zones = watchZonesApi(client);
    this.saved = savedApplicationsApi(client);
  }

  async fetchMyZones(): Promise<readonly WatchZoneSummary[]> {
    return this.zones.list();
  }

  async fetchApplicationsByZone(zoneId: WatchZoneId): Promise<readonly PlanningApplication[]> {
    return this.apps.getByZone(zoneId as string);
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
