import { useMemo } from 'react';
import { MapContainer, TileLayer, Marker, Polygon, Polyline, useMapEvents } from 'react-leaflet';
import type { LeafletMouseEvent, LeafletEvent, Marker as LeafletMarkerInstance } from 'leaflet';
import 'leaflet/dist/leaflet.css';
import type { Coordinates } from '../../domain/types';
import styles from './BoundaryMap.module.css';

const OSM_TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
const OSM_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

// Leaflet SVG layers accept raw colour values, not CSS custom properties —
// this hex MUST be kept in sync BY HAND with --tc-amber in tokens.css (dark
// theme value, #E9A620 / rgb(233, 166, 32)) whenever the palette changes.
// Mirrors ConfirmMap's convention for the confirm-step circle.
const CLOSED_RING_OPTIONS = {
  color: 'rgba(233, 166, 32, 0.8)',
  fillColor: 'rgb(233, 166, 32)',
  fillOpacity: 0.1,
  weight: 2,
};

// The in-progress (not yet closed) ring is drawn as a dashed line with no
// fill, so it reads as "still being drawn" rather than a completed shape.
const OPEN_RING_OPTIONS = {
  color: 'rgba(233, 166, 32, 0.8)',
  weight: 2,
  dashArray: '6 4',
};

interface Props {
  /** Initial map centre — the zone's postcode lookup or existing centroid. */
  centre: Coordinates;
  vertices: readonly Coordinates[];
  isClosed: boolean;
  onAddVertex: (coordinate: Coordinates) => void;
  onMoveVertex: (index: number, coordinate: Coordinates) => void;
  onCloseRing: () => void;
}

interface ClickCatcherProps {
  onAddVertex: (coordinate: Coordinates) => void;
}

/** Adds a vertex wherever the map is clicked. Renders nothing itself — it
 * only exists to host the `useMapEvents` hook, which must run inside the
 * `MapContainer` tree. */
function ClickCatcher({ onAddVertex }: ClickCatcherProps) {
  useMapEvents({
    click(event: LeafletMouseEvent) {
      onAddVertex({ latitude: event.latlng.lat, longitude: event.latlng.lng });
    },
  });
  return null;
}

export function BoundaryMap({
  centre,
  vertices,
  isClosed,
  onAddVertex,
  onMoveVertex,
  onCloseRing,
}: Props) {
  const positions = useMemo<[number, number][]>(
    () => vertices.map((vertex): [number, number] => [vertex.latitude, vertex.longitude]),
    [vertices],
  );
  const centrePosition: [number, number] = [centre.latitude, centre.longitude];

  return (
    <div className={styles.container}>
      <MapContainer
        center={centrePosition}
        zoom={15}
        style={{ height: '100%', width: '100%' }}
        zoomControl={false}
        attributionControl={true}
      >
        <TileLayer url={OSM_TILE_URL} attribution={OSM_ATTRIBUTION} />
        <ClickCatcher onAddVertex={onAddVertex} />

        {isClosed && positions.length >= 3 && (
          <Polygon positions={positions} pathOptions={CLOSED_RING_OPTIONS} />
        )}
        {!isClosed && positions.length >= 2 && (
          <Polyline positions={positions} pathOptions={OPEN_RING_OPTIONS} />
        )}

        {vertices.map((vertex, index) => (
          // Vertices are only ever appended/removed at the tail (drawing
          // order), never reordered — index is a stable identity here, and
          // using it (rather than the mutable lat/lng) is what lets a marker
          // keep its React identity while it's being dragged.
          // eslint-disable-next-line react/no-array-index-key
          <Marker
            key={index}
            position={[vertex.latitude, vertex.longitude]}
            draggable
            eventHandlers={{
              dragend: (event: LeafletEvent) => {
                const marker = event.target as LeafletMarkerInstance;
                const { lat, lng } = marker.getLatLng();
                onMoveVertex(index, { latitude: lat, longitude: lng });
              },
              // Only the first vertex closes the ring when clicked — every
              // other marker click is a no-op (dragging is how you adjust it).
              ...(index === 0 ? { click: () => onCloseRing() } : {}),
            }}
          />
        ))}
      </MapContainer>
    </div>
  );
}
