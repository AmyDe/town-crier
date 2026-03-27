import type { ApiClient } from './client';
import type { PlanningApplication } from '../domain/types';

export function applicationsApi(client: ApiClient) {
  return {
    getByAuthority: (authorityId: number) =>
      client.get<readonly PlanningApplication[]>('/v1/applications', { authorityId: String(authorityId) }),
    getByUid: (uid: string) =>
      client.get<PlanningApplication>(`/v1/applications/${uid}`),
  };
}
