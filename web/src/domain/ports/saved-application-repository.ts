import type { ApplicationUid, PlanningApplication, SavedApplication } from '../types';

export interface SavedApplicationRepository {
  listSaved(): Promise<readonly SavedApplication[]>;
  save(application: PlanningApplication): Promise<void>;
  remove(uid: ApplicationUid): Promise<void>;
}
