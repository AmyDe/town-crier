import type { ApiClient } from './client';
import type { AuthorityListItem, PlanningApplication } from '../domain/types';

interface UserApplicationAuthoritiesResponse {
  readonly authorities: readonly AuthorityListItem[];
  readonly count: number;
}

export function applicationsApi(client: ApiClient) {
  return {
    getMyAuthorities: () =>
      client
        .get<UserApplicationAuthoritiesResponse>('/v1/me/application-authorities')
        .then((r) => r.authorities),
    getByAuthority: (authorityId: number) =>
      client.get<readonly PlanningApplication[]>('/v1/applications', { authorityId: String(authorityId) }),
    getByUid: (uid: string) =>
      client.get<PlanningApplication>(`/v1/applications/${uid}`),
  };
}
