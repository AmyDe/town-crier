import type { ApplicationUid, SavedApplication } from '../types';

export interface SavedApplicationRepository {
  listSaved(): Promise<readonly SavedApplication[]>;
  save(uid: ApplicationUid): Promise<void>;
  remove(uid: ApplicationUid): Promise<void>;
}
