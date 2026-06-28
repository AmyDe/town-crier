import type { PlanningApplication } from '../types';

export interface ApplicationRepository {
  fetchApplication(authority: string, name: string): Promise<PlanningApplication>;
}
