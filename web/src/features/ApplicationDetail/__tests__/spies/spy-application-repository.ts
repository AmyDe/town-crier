import type { ApplicationRepository } from '../../../../domain/ports/application-repository';
import type { ApplicationUid, PlanningApplication } from '../../../../domain/types';
import { fullApplication } from '../fixtures/planning-application.fixtures';

export class SpyApplicationRepository implements ApplicationRepository {
  fetchApplicationCalls: ApplicationUid[] = [];
  fetchApplicationResult: PlanningApplication = fullApplication();
  fetchApplicationError: Error | null = null;

  async fetchApplication(uid: ApplicationUid): Promise<PlanningApplication> {
    this.fetchApplicationCalls.push(uid);
    if (this.fetchApplicationError) {
      throw this.fetchApplicationError;
    }
    return this.fetchApplicationResult;
  }
}
