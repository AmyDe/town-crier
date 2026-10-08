import { useState, useRef, useCallback, useEffect } from 'react';
import type { PostcodeLookupPort } from '../../../domain/ports/postcode-lookup-port';
import type { SearchLocation } from '../../../domain/ports/search-port';
import { extractErrorMessage } from '../../../utils/extractErrorMessage';

/** Debounce window between the last postcode keystroke and the client-side lookup. */
const POSTCODE_DEBOUNCE_MS = 400;
/** Shortest valid UK postcode ("M1 1AE" without its space) — avoids firing on every keystroke. */
const MIN_POSTCODE_LENGTH = 5;

const GEOLOCATION_UNAVAILABLE_MESSAGE = 'Geolocation is not supported by this browser.';
const GEOLOCATION_DENIED_MESSAGE =
  'Could not get your location. Check your browser permissions and try again.';

/**
 * ViewModel for the inline location picker: a debounced postcode lookup, "use
 * my location" and a map tap each call `onConfirm` as soon as they resolve.
 */
export function useLocationPicker(
  port: PostcodeLookupPort,
  onConfirm: (location: SearchLocation, label: string) => void,
) {
  const [postcode, setPostcodeRaw] = useState('');
  const [postcodeError, setPostcodeError] = useState<string | null>(null);
  const [isLookingUp, setIsLookingUp] = useState(false);
  const [geolocationError, setGeolocationError] = useState<string | null>(null);
  const [isLocating, setIsLocating] = useState(false);

  const requestIdRef = useRef(0);

  const setPostcode = useCallback((value: string) => {
    setPostcodeRaw(value);
    setPostcodeError(null);
  }, []);

  useEffect(() => {
    const trimmed = postcode.trim();
    if (trimmed.length < MIN_POSTCODE_LENGTH) {
      return;
    }

    const timer = setTimeout(() => {
      const requestId = ++requestIdRef.current;
      setIsLookingUp(true);
      setPostcodeError(null);
      void port
        .lookup(trimmed)
        .then((location) => {
          if (requestIdRef.current !== requestId) return;
          setIsLookingUp(false);
          onConfirm(location, trimmed.toUpperCase());
        })
        .catch((err: unknown) => {
          if (requestIdRef.current !== requestId) return;
          setIsLookingUp(false);
          setPostcodeError(extractErrorMessage(err, 'Could not look up that postcode.'));
        });
    }, POSTCODE_DEBOUNCE_MS);

    return () => clearTimeout(timer);
  }, [postcode, port, onConfirm]);

  const useMyLocation = useCallback(() => {
    setGeolocationError(null);

    if (!navigator.geolocation) {
      setGeolocationError(GEOLOCATION_UNAVAILABLE_MESSAGE);
      return;
    }

    setIsLocating(true);
    navigator.geolocation.getCurrentPosition(
      (position) => {
        setIsLocating(false);
        onConfirm(
          { lat: position.coords.latitude, lon: position.coords.longitude },
          'your location',
        );
      },
      () => {
        setIsLocating(false);
        setGeolocationError(GEOLOCATION_DENIED_MESSAGE);
      },
    );
  }, [onConfirm]);

  const selectMapPoint = useCallback(
    (lat: number, lon: number) => {
      onConfirm({ lat, lon }, 'custom pin');
    },
    [onConfirm],
  );

  return {
    postcode,
    setPostcode,
    postcodeError,
    isLookingUp,
    geolocationError,
    isLocating,
    useMyLocation,
    selectMapPoint,
  };
}
