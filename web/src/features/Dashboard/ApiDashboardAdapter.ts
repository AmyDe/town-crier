import type { ApiClient } from '../../api/client';
import type { DashboardPort } from '../../domain/ports/dashboard-port';
import type { WatchZoneId, PlanningApplication, PlanningApplicationSummary, WatchZoneSummary } from '../../domain/types';
import { watchZonesApi } from '../../api/watchZones';
import { applicationsApi } from '../../api/applications';

function toSummary(app: PlanningApplication): PlanningApplicationSummary {
  return {
    uid: app.uid,
    name: app.name,
    address: app.address,
    postcode: app.postcode,
    description: app.description,
    appType: app.appType,
    appState: app.appState,
    areaName: app.areaName,
    startDate: app.startDate,
    url: app.url,
    latestUnreadEvent: app.latestUnreadEvent,
  };
}

export class ApiDashboardAdapter implements DashboardPort {
  private readonly watchZones;
  private readonly applications;

  constructor(client: ApiClient) {
    this.watchZones = watchZonesApi(client);
    this.applications = applicationsApi(client);
  }

  async fetchWatchZones(): Promise<readonly WatchZoneSummary[]> {
    return this.watchZones.list();
  }

  async fetchRecentApplications(zoneId: WatchZoneId): Promise<readonly PlanningApplicationSummary[]> {
    const apps = await this.applications.getByZone(zoneId as string);
    return apps.map(toSummary);
  }
}
