import type { ApplicationUid, WatchZoneId, WatchZoneSummary, PlanningApplication, SavedApplication } from '../../../../domain/types';
import type { MapPort } from '../../../../domain/ports/map-port';

export class SpyMapPort implements MapPort {
  fetchMyZonesCalls = 0;
  fetchMyZonesResult: readonly WatchZoneSummary[] = [];
  fetchMyZonesError: Error | null = null;

  async fetchMyZones(): Promise<readonly WatchZoneSummary[]> {
    this.fetchMyZonesCalls++;
    if (this.fetchMyZonesError) {
      throw this.fetchMyZonesError;
    }
    return this.fetchMyZonesResult;
  }

  fetchApplicationsByZoneCalls: WatchZoneId[] = [];
  fetchApplicationsByZoneResults: Map<string, readonly PlanningApplication[]> = new Map();
  fetchApplicationsByZoneError: Error | null = null;

  async fetchApplicationsByZone(zoneId: WatchZoneId): Promise<readonly PlanningApplication[]> {
    this.fetchApplicationsByZoneCalls.push(zoneId);
    if (this.fetchApplicationsByZoneError) {
      throw this.fetchApplicationsByZoneError;
    }
    return this.fetchApplicationsByZoneResults.get(zoneId as string) ?? [];
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

  saveApplicationCalls: PlanningApplication[] = [];
  saveApplicationError: Error | null = null;

  async saveApplication(application: PlanningApplication): Promise<void> {
    this.saveApplicationCalls.push(application);
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
