import { Link } from 'react-router-dom';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import type { MapPort } from '../../domain/ports/map-port';
import { useMapData } from './useMapData';
import styles from './MapPage.module.css';
import 'leaflet/dist/leaflet.css';

const OSM_TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
const OSM_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

const DEFAULT_CENTER: [number, number] = [51.5074, -0.1278];
const DEFAULT_ZOOM = 13;

interface Props {
  port: MapPort;
}

export function MapPage({ port }: Props) {
  const { zones, applications, isLoading, error } = useMapData(port);

  if (isLoading) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Map</h1>
        <div className={styles.loading}>Loading...</div>
      </div>
    );
  }

  if (error !== null) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Map</h1>
        <div className={styles.error}>{error}</div>
      </div>
    );
  }

  const center: [number, number] =
    zones.length > 0
      ? [zones[0]!.latitude, zones[0]!.longitude]
      : DEFAULT_CENTER;

  const markableApplications = applications.filter(
    app => app.latitude !== null && app.longitude !== null,
  );

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Map</h1>
      <div className={styles.mapWrapper}>
        <MapContainer
          center={center}
          zoom={DEFAULT_ZOOM}
          style={{ height: '100%', width: '100%' }}
        >
          <TileLayer url={OSM_TILE_URL} attribution={OSM_ATTRIBUTION} />
          {markableApplications.map(app => (
            <Marker
              key={app.uid}
              position={[app.latitude!, app.longitude!]}
            >
              <Popup>
                <p className={styles.popupDescription}>{app.description}</p>
                <p className={styles.popupAddress}>{app.address}</p>
                <Link
                  className={styles.popupLink}
                  to={`/applications/${app.uid}`}
                >
                  View details
                </Link>
              </Popup>
            </Marker>
          ))}
        </MapContainer>
      </div>
    </div>
  );
}
