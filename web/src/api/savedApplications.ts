import type { ApiClient } from './client';
import type { SavedApplication } from '../domain/types';

export function savedApplicationsApi(client: ApiClient) {
  return {
    list: () => client.get<readonly SavedApplication[]>('/v1/me/saved-applications'),
    save: (applicationUid: string) =>
      client.put(`/v1/me/saved-applications/${applicationUid}`),
    remove: (applicationUid: string) =>
      client.delete(`/v1/me/saved-applications/${applicationUid}`),
  };
}
