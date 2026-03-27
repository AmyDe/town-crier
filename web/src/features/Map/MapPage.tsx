import { Link } from 'react-router-dom';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import 'leaflet/dist/leaflet.css';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import type { MapApplicationsPort } from '../../domain/ports/map-applications-port';
import { useMapData } from './useMapData';
import styles from './MapPage.module.css';

const OSM_TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
const OSM_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';
const DEFAULT_ZOOM = 13;

interface Props {
  watchZoneRepo: WatchZoneRepository;
  applicationsPort: MapApplicationsPort;
}

export function MapPage({ watchZoneRepo, applicationsPort }: Props) {
  const { applications, center, isLoading, error } = useMapData(
    watchZoneRepo,
    applicationsPort,
  );

  if (isLoading) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Map</h1>
        <div className={styles.loading}>Loading map data...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className={styles.container}>
        <h1 className={styles.heading}>Map</h1>
        <div className={styles.error}>{error}</div>
      </div>
    );
  }

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Map</h1>
      <div className={styles.mapWrapper}>
        <MapContainer
          center={[center.lat, center.lng]}
          zoom={DEFAULT_ZOOM}
          style={{ height: '100%', width: '100%' }}
        >
          <TileLayer url={OSM_TILE_URL} attribution={OSM_ATTRIBUTION} />
          {applications.map((app) => (
            <Marker
              key={app.uid}
              position={[app.latitude!, app.longitude!]}
            >
              <Popup>
                <p className={styles.popupName}>{app.name}</p>
                <p className={styles.popupAddress}>{app.address}</p>
                <p className={styles.popupDescription}>{app.description}</p>
                <span className={styles.popupStatus}>{app.appState}</span>
                <br />
                <Link
                  to={`/applications/${app.uid}`}
                  className={styles.popupLink}
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
