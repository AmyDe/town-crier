import type { ApiClient } from './client';
import type { DesignationContext } from '../domain/types';

export function designationsApi(client: ApiClient) {
  return {
    getContext: (latitude: number, longitude: number) =>
      client.get<DesignationContext>('/v1/designations', {
        latitude: String(latitude),
        longitude: String(longitude),
      }),
  };
}
