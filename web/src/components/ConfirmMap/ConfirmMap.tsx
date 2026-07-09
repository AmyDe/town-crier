import { useEffect } from 'react';
import { MapContainer, TileLayer, Marker, Circle, useMap } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import styles from './ConfirmMap.module.css';

const OSM_TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
const OSM_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

// Leaflet SVG layers accept raw colour values, not CSS custom properties —
// this hex MUST be kept in sync BY HAND with --tc-amber in tokens.css (dark
// theme value, #E9A620 / rgb(233, 166, 32)) whenever the palette changes.
// Public Notice component language: a dashed amber ring at low fill opacity,
// not a solid highlight.
const CIRCLE_OPTIONS = {
  color: 'rgba(233, 166, 32, 0.8)',
  fillColor: 'rgb(233, 166, 32)',
  fillOpacity: 0.08,
  weight: 2,
  dashArray: '6 4',
};

interface Props {
  latitude: number;
  longitude: number;
  radiusMetres: number;
}

function FitToCircle({ latitude, longitude, radiusMetres }: Props) {
  const map = useMap();

  useEffect(() => {
    const bounds = L.latLng(latitude, longitude).toBounds(radiusMetres * 2);
    map.fitBounds(bounds.pad(0.1));
  }, [map, latitude, longitude, radiusMetres]);

  return null;
}

export function ConfirmMap({ latitude, longitude, radiusMetres }: Props) {
  const centre: [number, number] = [latitude, longitude];

  return (
    <div className={styles.container}>
      <MapContainer
        center={centre}
        zoom={13}
        style={{ height: '100%', width: '100%' }}
        zoomControl={false}
        attributionControl={true}
      >
        <TileLayer url={OSM_TILE_URL} attribution={OSM_ATTRIBUTION} />
        <Marker position={centre} />
        <Circle center={centre} radius={radiusMetres} pathOptions={CIRCLE_OPTIONS} />
        <FitToCircle latitude={latitude} longitude={longitude} radiusMetres={radiusMetres} />
      </MapContainer>
    </div>
  );
}
