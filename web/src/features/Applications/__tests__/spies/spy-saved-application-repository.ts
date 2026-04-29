import type { ApplicationUid, SavedApplication } from '../../../../domain/types';
import type { SavedApplicationRepository } from '../../../../domain/ports/saved-application-repository';

export class SpySavedApplicationRepository implements SavedApplicationRepository {
  listSavedCalls = 0;
  listSavedResult: readonly SavedApplication[] = [];
  listSavedError: Error | null = null;

  async listSaved(): Promise<readonly SavedApplication[]> {
    this.listSavedCalls += 1;
    if (this.listSavedError) {
      throw this.listSavedError;
    }
    return this.listSavedResult;
  }

  saveCalls: ApplicationUid[] = [];

  async save(uid: ApplicationUid): Promise<void> {
    this.saveCalls.push(uid);
  }

  removeCalls: ApplicationUid[] = [];
  removeError: Error | null = null;

  async remove(uid: ApplicationUid): Promise<void> {
    this.removeCalls.push(uid);
    if (this.removeError) {
      throw this.removeError;
    }
  }
}
