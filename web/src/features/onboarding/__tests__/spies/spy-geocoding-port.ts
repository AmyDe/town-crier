import type { GeocodeResult } from '../../../../domain/types';
import type { GeocodingPort } from '../../../../domain/ports/geocoding-port';

export class SpyGeocodingPort implements GeocodingPort {
  geocodeCalls: string[] = [];
  geocodeResult: GeocodeResult = { latitude: 52.2053, longitude: 0.1218 };
  geocodeError: Error | null = null;

  async geocode(postcode: string): Promise<GeocodeResult> {
    this.geocodeCalls.push(postcode);
    if (this.geocodeError) {
      throw this.geocodeError;
    }
    return this.geocodeResult;
  }
}
