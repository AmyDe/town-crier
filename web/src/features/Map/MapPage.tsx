import { useCallback, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { MapContainer, TileLayer, Marker, useMap, useMapEvents } from 'react-leaflet';
import type { MapPort, MapBounds } from '../../domain/ports/map-port';
import type { ApplicationStatus, ClusterMember, MapCluster, WatchZoneSummary } from '../../domain/types';
import { clusterMemberStatus } from '../../domain/types';
import { useMapData } from './useMapData';
import { countBubbleIcon, statusPinIcon } from './markerIcons';
import styles from './MapPage.module.css';
import 'leaflet/dist/leaflet.css';
import './leaflet-overrides.css';

const OSM_TILE_URL = 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
const OSM_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

const UK_CENTER: [number, number] = [54.5, -2.5];
const ZONE_ZOOM = 13;
const MAX_ZOOM = 18;

interface StatusChip {
  readonly label: string;
  readonly status: ApplicationStatus | null;
}

const STATUS_CHIPS: readonly StatusChip[] = [
  { label: 'All', status: null },
  { label: 'Pending', status: 'Undecided' },
  { label: 'Granted', status: 'Permitted' },
  { label: 'Granted with conditions', status: 'Conditions' },
  { label: 'Refused', status: 'Rejected' },
  { label: 'Withdrawn', status: 'Withdrawn' },
  { label: 'Appealed', status: 'Appealed' },
];

function clusterKey(cluster: MapCluster): string {
  if (cluster.member) {
    return `${cluster.member.authority}/${cluster.member.name}`;
  }
  return `cluster:${cluster.latitude}:${cluster.longitude}:${cluster.count}`;
}

interface ClusterLayerProps {
  readonly clusters: readonly MapCluster[];
  readonly onRegionChange: (bounds: MapBounds, zoom: number) => void;
  readonly onSelectMember: (member: ClusterMember) => void;
}

/**
 * Renders the cluster aggregates as Leaflet markers and reports the viewport.
 * Lives inside `MapContainer` so it can read the map via `useMap`: it reports
 * the visible rect on mount and on every `moveend`/`zoomend` (the hook debounces
 * the resulting refetch), zooms in on a count-bubble tap, and routes a single
 * pin tap to a point-read.
 */
function ClusterLayer({ clusters, onRegionChange, onSelectMember }: ClusterLayerProps) {
  const map = useMap();

  const report = useCallback(() => {
    const bounds = map.getBounds();
    onRegionChange(
      {
        west: bounds.getWest(),
        south: bounds.getSouth(),
        east: bounds.getEast(),
        north: bounds.getNorth(),
      },
      map.getZoom(),
    );
  }, [map, onRegionChange]);

  useMapEvents({ moveend: report, zoomend: report });

  useEffect(() => {
    report();
  }, [report]);

  return (
    <>
      {clusters.map((cluster) => {
        const position: [number, number] = [cluster.latitude, cluster.longitude];
        if (cluster.count > 1) {
          return (
            <Marker
              key={clusterKey(cluster)}
              position={position}
              icon={countBubbleIcon(cluster.count)}
              eventHandlers={{
                click: () =>
                  map.setView(position, Math.min(map.getZoom() + 2, MAX_ZOOM)),
              }}
            />
          );
        }
        const status = clusterMemberStatus(cluster) ?? '';
        const member = cluster.member;
        return (
          <Marker
            key={clusterKey(cluster)}
            position={position}
            icon={statusPinIcon(status)}
            eventHandlers={{ click: () => member && onSelectMember(member) }}
          />
        );
      })}
    </>
  );
}

interface Props {
  port: MapPort;
}

export function MapPage({ port }: Props) {
  const navigate = useNavigate();
  const {
    zones,
    selectedZone,
    clusters,
    selectedStatusFilter,
    isLoading,
    error,
    onRegionChange,
    setStatusFilter,
    selectZone,
    resolveMember,
  } = useMapData(port);

  const handleSelectMember = useCallback(
    async (member: ClusterMember) => {
      const application = await resolveMember(member);
      if (application) {
        navigate(`/applications/${application.uid}`, {
          state: {
            authority: String(application.areaId),
            name: application.name,
          },
        });
      }
    },
    [resolveMember, navigate],
  );

  function handleZoneChange(value: string) {
    const zone = zones.find((z) => z.id === value);
    if (zone) {
      selectZone(zone);
    }
  }

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

  const center: [number, number] = selectedZone
    ? [selectedZone.latitude, selectedZone.longitude]
    : UK_CENTER;

  return (
    <div className={styles.container}>
      <h1 className={styles.heading}>Map</h1>

      {zones.length > 1 && (
        <label className={styles.zonePicker}>
          <span className={styles.zonePickerLabel}>Watch zone</span>
          <select
            aria-label="Watch zone"
            className={styles.zoneSelect}
            value={selectedZone?.id ?? ''}
            onChange={(e) => handleZoneChange(e.target.value)}
          >
            {zones.map((zone: WatchZoneSummary) => (
              <option key={zone.id} value={zone.id}>
                {zone.name}
              </option>
            ))}
          </select>
        </label>
      )}

      {selectedZone && (
        <div className={styles.statusFilters} role="group" aria-label="Filter by status">
          {STATUS_CHIPS.map((chip) => {
            const isActive = selectedStatusFilter === chip.status;
            return (
              <button
                key={chip.label}
                type="button"
                className={`${styles.chip} ${isActive ? styles.chipActive : ''}`}
                aria-pressed={isActive}
                onClick={() => setStatusFilter(chip.status)}
              >
                {chip.label}
              </button>
            );
          })}
        </div>
      )}

      <div className={styles.mapWrapper}>
        <MapContainer
          key={selectedZone?.id ?? 'no-zone'}
          center={center}
          zoom={selectedZone ? ZONE_ZOOM : 6}
          style={{ height: '100%', width: '100%' }}
        >
          <TileLayer url={OSM_TILE_URL} attribution={OSM_ATTRIBUTION} />
          <ClusterLayer
            clusters={clusters}
            onRegionChange={onRegionChange}
            onSelectMember={handleSelectMember}
          />
        </MapContainer>
      </div>
    </div>
  );
}
