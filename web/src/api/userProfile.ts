import type { ApiClient } from './client';
import type { UserProfile, UpdateProfileRequest } from '../domain/types';

export function userProfileApi(client: ApiClient) {
  return {
    create: () => client.post<UserProfile>('/v1/me'),
    get: () => client.get<UserProfile>('/v1/me'),
    update: (data: UpdateProfileRequest) => client.patch<UserProfile>('/v1/me', data),
    delete: () => client.delete('/v1/me'),
  };
}
