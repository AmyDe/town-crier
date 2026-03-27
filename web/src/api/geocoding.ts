import type { ApiClient } from './client';
import type { GeocodeResult } from '../domain/types';

export function geocodingApi(client: ApiClient) {
  return {
    geocode: (postcode: string) =>
      client.get<GeocodeResult>(`/v1/geocode/${encodeURIComponent(postcode)}`),
  };
}
