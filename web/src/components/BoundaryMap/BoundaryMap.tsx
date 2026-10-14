import { useMemo, useState } from 'react';
import { MapContainer, TileLayer, Marker, Polygon, Polyline, useMapEvents } from 'react-leaflet';
import type {
  LeafletMouseEvent,
  LeafletEvent,
  Marker as LeafletMarkerInstance,
  LatLngBoundsExpression,
} from 'leaflet';
import 'leaflet/dist/leaflet.css';
import type { Coordinates } from '../../domain/types';
import { OSM_ATTRIBUTION, OSM_TILE_URL } from '../osmTiles';
import { computeBoundaryMapView } from './boundaryMapView';
import styles from './BoundaryMap.module.css';

// Leaflet SVG layers accept raw colour values, not CSS custom properties —
// this hex MUST be kept in sync BY HAND with --tc-amber in tokens.css (dark
// theme value, #E9A620 / rgb(233, 166, 32)) whenever the palette changes.
const CLOSED_RING_OPTIONS = {
  color: 'rgba(233, 166, 32, 0.8)',
  fillColor: 'rgb(233, 166, 32)',
  fillOpacity: 0.1,
  weight: 2,
};

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
  /** Reopens a closed ring, or removes the last vertex while still drawing. */
  onUndo: () => void;
  /** Clears the ring entirely so drawing can start over. */
  onReset: () => void;
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
  onUndo,
  onReset,
}: Props) {
  const positions = useMemo<[number, number][]>(
    () => vertices.map((vertex): [number, number] => [vertex.latitude, vertex.longitude]),
    [vertices],
  );
  const hasVertices = vertices.length > 0;

  // Computed once on mount: a new shape keeps the fixed view for the whole
  // drawing session even as vertices are added.
  const [mapView] = useState(() => computeBoundaryMapView(vertices, centre));
  const mapViewProps =
    mapView.kind === 'fixed'
      ? {
          center: [mapView.center.latitude, mapView.center.longitude] as [number, number],
          zoom: mapView.zoom,
        }
      : {
          bounds: [
            [mapView.southWest.latitude, mapView.southWest.longitude],
            [mapView.northEast.latitude, mapView.northEast.longitude],
          ] as LatLngBoundsExpression,
        };

  return (
    <div>
      <div className={styles.controls}>
        <button
          type="button"
          className={styles.controlButton}
          onClick={onUndo}
          disabled={!hasVertices}
        >
          Undo
        </button>
        <button
          type="button"
          className={styles.controlButton}
          onClick={onReset}
          disabled={!hasVertices}
        >
          Start over
        </button>
      </div>
      <div className={styles.container}>
        <MapContainer
          {...mapViewProps}
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
            // Vertices only change at the tail, so index is a stable key and
            // keeps a marker's identity while it is dragged.
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
                ...(index === 0 ? { click: () => onCloseRing() } : {}),
              }}
            />
          ))}
        </MapContainer>
      </div>
    </div>
  );
}
