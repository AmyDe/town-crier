import type { ApiClient } from './client';
import type { PlanningApplication, SavedApplication } from '../domain/types';

export function savedApplicationsApi(client: ApiClient) {
  return {
    list: () => client.get<readonly SavedApplication[]>('/v1/me/saved-applications'),
    save: (application: PlanningApplication) =>
      client.put(`/v1/me/saved-applications/${application.uid}`, application),
    remove: (applicationUid: string) =>
      client.delete(`/v1/me/saved-applications/${applicationUid}`),
  };
}
