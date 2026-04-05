import { useMemo } from 'react';
import { Link } from 'react-router-dom';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import type { MapPort } from '../../domain/ports/map-port';
import { useMapData } from './useMapData';
import { savedMarkerIcon, unsavedMarkerIcon } from './markerIcons';
import { FitBounds } from './FitBounds';
import { BookmarkButton } from './BookmarkButton';
import styles from './MapPage.module.css';
import 'leaflet/dist/leaflet.css';

const OSM_TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
const OSM_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

const UK_CENTER: [number, number] = [51.5074, -0.1278];
const DEFAULT_ZOOM = 13;

interface Props {
  port: MapPort;
}

export function MapPage({ port }: Props) {
  const { applications, savedUids, isLoading, error, saveApplication, unsaveApplication } =
    useMapData(port);

  const markableApplications = useMemo(
    () => applications.filter(app => app.latitude !== null && app.longitude !== null),
    [applications],
  );

  const fitPositions = useMemo(() => {
    const savedApps = markableApplications.filter(app => savedUids.has(app.uid));
    const targets = savedApps.length > 0 ? savedApps : markableApplications;
    return targets.map(app => [app.latitude!, app.longitude!] as [number, number]);
  }, [markableApplications, savedUids]);

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

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Map</h1>
      <div className={styles.mapWrapper}>
        <MapContainer
          center={UK_CENTER}
          zoom={DEFAULT_ZOOM}
          style={{ height: '100%', width: '100%' }}
        >
          <TileLayer url={OSM_TILE_URL} attribution={OSM_ATTRIBUTION} />
          <FitBounds positions={fitPositions} />
          {markableApplications.map(app => {
            const isSaved = savedUids.has(app.uid);
            return (
              <Marker
                key={app.uid}
                position={[app.latitude!, app.longitude!]}
                icon={isSaved ? savedMarkerIcon : unsavedMarkerIcon}
              >
                <Popup>
                  <div className={styles.popupHeader}>
                    <p className={styles.popupDescription}>{app.description}</p>
                    <BookmarkButton
                      isSaved={isSaved}
                      onToggle={() =>
                        isSaved ? unsaveApplication(app.uid) : saveApplication(app.uid)
                      }
                    />
                  </div>
                  <p className={styles.popupAddress}>{app.address}</p>
                  <Link
                    className={styles.popupLink}
                    to={`/applications/${app.uid}`}
                  >
                    View details
                  </Link>
                </Popup>
              </Marker>
            );
          })}
        </MapContainer>
      </div>
    </div>
  );
}
