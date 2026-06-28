import type { ApplicationRepository } from '../../../../domain/ports/application-repository';
import type { PlanningApplication } from '../../../../domain/types';
import { fullApplication } from '../fixtures/planning-application.fixtures';

export interface FetchApplicationCall {
  readonly authority: string;
  readonly name: string;
}

export class SpyApplicationRepository implements ApplicationRepository {
  fetchApplicationCalls: FetchApplicationCall[] = [];
  fetchApplicationResult: PlanningApplication = fullApplication();
  fetchApplicationError: Error | null = null;

  async fetchApplication(authority: string, name: string): Promise<PlanningApplication> {
    this.fetchApplicationCalls.push({ authority, name });
    if (this.fetchApplicationError) {
      throw this.fetchApplicationError;
    }
    return this.fetchApplicationResult;
  }
}
