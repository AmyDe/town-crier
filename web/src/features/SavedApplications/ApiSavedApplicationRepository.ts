import type { ApiClient } from '../../api/client';
import type { ApplicationUid, SavedApplication } from '../../domain/types';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
import { savedApplicationsApi } from '../../api/savedApplications';

export class ApiSavedApplicationRepository implements SavedApplicationRepository {
  private readonly api: ReturnType<typeof savedApplicationsApi>;

  constructor(client: ApiClient) {
    this.api = savedApplicationsApi(client);
  }

  async listSaved(): Promise<readonly SavedApplication[]> {
    return this.api.list();
  }

  async save(uid: ApplicationUid): Promise<void> {
    return this.api.save(uid);
  }

  async remove(uid: ApplicationUid): Promise<void> {
    return this.api.remove(uid);
  }
}
