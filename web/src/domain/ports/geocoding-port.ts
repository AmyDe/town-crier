import type { GeocodeResult } from '../types';

export interface GeocodingPort {
  geocode(postcode: string): Promise<GeocodeResult>;
}
