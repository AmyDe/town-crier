import type { DesignationRepository } from '../../../../domain/ports/designation-repository';
import type { DesignationContext } from '../../../../domain/types';
import { conservationAreaDesignation } from '../fixtures/designation-context.fixtures';

interface FetchDesignationsCall {
  latitude: number;
  longitude: number;
}

export class SpyDesignationRepository implements DesignationRepository {
  fetchDesignationsCalls: FetchDesignationsCall[] = [];
  fetchDesignationsResult: DesignationContext = conservationAreaDesignation();
  fetchDesignationsError: Error | null = null;

  async fetchDesignations(latitude: number, longitude: number): Promise<DesignationContext> {
    this.fetchDesignationsCalls.push({ latitude, longitude });
    if (this.fetchDesignationsError) {
      throw this.fetchDesignationsError;
    }
    return this.fetchDesignationsResult;
  }
}
