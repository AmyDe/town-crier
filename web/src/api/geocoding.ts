import type { ApiClient } from './client';
import type { GeocodeResult } from '../domain/types';

interface GeocodeResponse {
  readonly coordinates: GeocodeResult;
}

export function geocodingApi(client: ApiClient) {
  return {
    geocode: async (postcode: string): Promise<GeocodeResult> => {
      const response = await client.get<GeocodeResponse>(`/v1/geocode/${encodeURIComponent(postcode)}`);
      return response.coordinates;
    },
  };
}
