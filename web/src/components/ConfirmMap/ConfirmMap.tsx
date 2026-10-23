import { useEffect } from 'react';
import { MapContainer, TileLayer, Marker, Circle, useMap } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import { OSM_ATTRIBUTION, OSM_TILE_URL } from '../osmTiles';
import styles from './ConfirmMap.module.css';

// Leaflet SVG layers need raw colour values, not CSS custom properties: keep in
// sync by hand with --tc-amber (dark theme) in tokens.css.
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
