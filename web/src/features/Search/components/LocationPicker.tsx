import { MapContainer, TileLayer, Marker, useMapEvents } from 'react-leaflet';
import type { PostcodeLookupPort } from '../../../domain/ports/postcode-lookup-port';
import type { SearchLocation } from '../../../domain/ports/search-port';
import { OSM_TILE_URL, OSM_ATTRIBUTION } from '../../../components/osmTiles';
import { useLocationPicker } from './useLocationPicker';
import styles from './LocationPicker.module.css';
import 'leaflet/dist/leaflet.css';

const UK_CENTER: [number, number] = [54.5, -2.5];
const DEFAULT_ZOOM = 6;
const PICKED_ZOOM = 15;

interface MapClickEvent {
  readonly latlng: { readonly lat: number; readonly lng: number };
}

interface TapToPlaceProps {
  readonly onSelect: (lat: number, lon: number) => void;
}

/** Lives inside `MapContainer` so a plain tap anywhere drops a pin there. */
function TapToPlace({ onSelect }: TapToPlaceProps) {
  useMapEvents({
    click: (event: MapClickEvent) => onSelect(event.latlng.lat, event.latlng.lng),
  });
  return null;
}

interface Props {
  readonly postcodePort: PostcodeLookupPort;
  /** Prefills the map centre/marker when reopened via "Change". */
  readonly initialLocation: SearchLocation | null;
  readonly onConfirm: (location: SearchLocation, label: string) => void;
}

/**
 * Inline location picker for the mandatory `/search` location gate (GH#863,
 * tc-rrv7i.2) — the "disabled bar + inline gate" pattern, never a full-page
 * picker or a blocking modal. Offers three equally-valid ways to resolve a
 * location, each confirming immediately on success: a debounced client-side
 * postcode lookup (`useLocationPicker` — never routed through our own API),
 * "use my location" via `navigator.geolocation`, and a tap anywhere on the
 * map. No radius UI: this component only ever resolves a point.
 */
export function LocationPicker({ postcodePort, initialLocation, onConfirm }: Props) {
  const {
    postcode,
    setPostcode,
    postcodeError,
    isLookingUp,
    geolocationError,
    isLocating,
    useMyLocation,
    selectMapPoint,
  } = useLocationPicker(postcodePort, onConfirm);

  const center: [number, number] = initialLocation
    ? [initialLocation.lat, initialLocation.lon]
    : UK_CENTER;
  const zoom = initialLocation ? PICKED_ZOOM : DEFAULT_ZOOM;

  return (
    <div className={styles.container}>
      <div className={styles.field}>
        <label className={styles.label} htmlFor="location-picker-postcode">
          Postcode
        </label>
        <input
          id="location-picker-postcode"
          type="text"
          className={styles.input}
          placeholder="e.g. SW1A 1AA"
          value={postcode}
          onChange={(event) => setPostcode(event.target.value)}
        />
        {isLookingUp && <p className={styles.status}>Looking up…</p>}
        {postcodeError !== null && (
          <p className={styles.error} role="alert">
            {postcodeError}
          </p>
        )}
      </div>

      <button
        type="button"
        className={styles.locateButton}
        onClick={useMyLocation}
        disabled={isLocating}
      >
        {isLocating ? 'Locating…' : 'Use my location'}
      </button>
      {geolocationError !== null && (
        <p className={styles.error} role="alert">
          {geolocationError}
        </p>
      )}

      <p className={styles.hint}>Or tap the map to drop a pin</p>
      <div className={styles.mapWrapper}>
        <MapContainer center={center} zoom={zoom} style={{ height: '100%', width: '100%' }}>
          <TileLayer url={OSM_TILE_URL} attribution={OSM_ATTRIBUTION} />
          {initialLocation && <Marker position={[initialLocation.lat, initialLocation.lon]} />}
          <TapToPlace onSelect={selectMapPoint} />
        </MapContainer>
      </div>
    </div>
  );
}
