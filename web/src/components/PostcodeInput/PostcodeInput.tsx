import type { GeocodeResult } from '../../domain/types';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import { usePostcodeGeocode } from './usePostcodeGeocode';
import styles from './PostcodeInput.module.css';

interface Props {
  geocodingPort: GeocodingPort;
  onGeocode: (result: GeocodeResult, postcode: string) => void;
}

export function PostcodeInput({ geocodingPort, onGeocode }: Props) {
  const { postcode, setPostcode, isGeocoding, error, lookup } =
    usePostcodeGeocode(geocodingPort);

  async function handleLookup() {
    const result = await lookup();
    if (result) {
      onGeocode(result, postcode);
    }
  }

  return (
    <div className={styles.container}>
      <div className={styles.row}>
        <input
          type="text"
          role="textbox"
          aria-label="Postcode"
          className={styles.input}
          value={postcode}
          onChange={(e) => setPostcode(e.target.value)}
          placeholder="e.g. SW1A 1AA"
        />
        <button
          type="button"
          className={styles.button}
          onClick={handleLookup}
          disabled={isGeocoding}
        >
          Look up
        </button>
      </div>
      {error && (
        <p className={styles.error} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
