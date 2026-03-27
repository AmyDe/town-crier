import type { SavedApplicationRepository } from '../../../../domain/ports/saved-application-repository';
import type { ApplicationUid, SavedApplication } from '../../../../domain/types';

export class SpySavedApplicationRepository implements SavedApplicationRepository {
  listSavedCalls = 0;
  listSavedResult: readonly SavedApplication[] = [];
  listSavedError: Error | null = null;

  saveCalls: ApplicationUid[] = [];
  saveError: Error | null = null;

  removeCalls: ApplicationUid[] = [];
  removeError: Error | null = null;

  async listSaved(): Promise<readonly SavedApplication[]> {
    this.listSavedCalls += 1;
    if (this.listSavedError) {
      throw this.listSavedError;
    }
    return this.listSavedResult;
  }

  async save(uid: ApplicationUid): Promise<void> {
    this.saveCalls.push(uid);
    if (this.saveError) {
      throw this.saveError;
    }
  }

  async remove(uid: ApplicationUid): Promise<void> {
    this.removeCalls.push(uid);
    if (this.removeError) {
      throw this.removeError;
    }
  }
}
