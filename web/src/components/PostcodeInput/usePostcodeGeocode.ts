import { useState, useCallback } from 'react';
import type { GeocodeResult } from '../../domain/types';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

export function usePostcodeGeocode(port: GeocodingPort) {
  const [postcode, setPostcodeRaw] = useState('');
  const [isGeocoding, setIsGeocoding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const setPostcode = useCallback((value: string) => {
    setPostcodeRaw(value);
    setError(null);
  }, []);

  const lookup = useCallback(async (): Promise<GeocodeResult | null> => {
    setIsGeocoding(true);
    setError(null);
    try {
      const result = await port.geocode(postcode);
      setIsGeocoding(false);
      return result;
    } catch (err) {
      const message = extractErrorMessage(err, 'Geocode failed');
      setError(message);
      setIsGeocoding(false);
      return null;
    }
  }, [port, postcode]);

  return { postcode, setPostcode, isGeocoding, error, lookup };
}
