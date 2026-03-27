import type { ApplicationUid, PlanningApplication } from '../types';

export interface ApplicationRepository {
  fetchApplication(uid: ApplicationUid): Promise<PlanningApplication>;
}
