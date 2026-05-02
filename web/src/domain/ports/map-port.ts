import type { ApplicationUid, WatchZoneId, WatchZoneSummary, PlanningApplication, SavedApplication } from '../types';

export interface MapPort {
  fetchMyZones(): Promise<readonly WatchZoneSummary[]>;
  fetchApplicationsByZone(zoneId: WatchZoneId): Promise<readonly PlanningApplication[]>;
  fetchSavedApplications(): Promise<readonly SavedApplication[]>;
  saveApplication(application: PlanningApplication): Promise<void>;
  unsaveApplication(uid: ApplicationUid): Promise<void>;
}
