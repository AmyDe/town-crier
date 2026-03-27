import type { ApiClient } from './client';
import type { AuthoritiesResult, AuthorityDetail } from '../domain/types';

export function authoritiesApi(client: ApiClient) {
  return {
    list: (search?: string) =>
      client.get<AuthoritiesResult>('/v1/authorities', search ? { search } : undefined),
    getById: (id: number) =>
      client.get<AuthorityDetail>(`/v1/authorities/${id}`),
  };
}
