import type {
  ApplicationStatus,
  ClusterMember,
  MapCluster,
  PlanningApplication,
  WatchZoneId,
  WatchZoneSummary,
} from '../types';

/** A geographic rectangle in WGS84 decimal degrees (a Leaflet map's bounds). */
export interface MapBounds {
  readonly west: number;
  readonly south: number;
  readonly east: number;
  readonly north: number;
}

/**
 * Drives the watch-zone map with server-computed cluster aggregates (GH#698).
 * Instead of eager-draining every application across every zone, the map fetches
 * only the clusters inside the current viewport of the active zone and refetches
 * (debounced) on pan/zoom. A single-member cell resolves to a full application
 * via a one-row point read.
 */
export interface MapPort {
  fetchMyZones(): Promise<readonly WatchZoneSummary[]>;
  /**
   * Fetches the cluster aggregates for the visible `bounds` at `zoom` of one
   * zone. `status`, when non-null, filters server-side by PlanIt `app_state`.
   */
  fetchClusters(
    zoneId: WatchZoneId,
    bounds: MapBounds,
    zoom: number,
    status: ApplicationStatus | null,
  ): Promise<readonly MapCluster[]>;
  /** Point-reads the full application identified by a single-member cell. */
  fetchApplicationByMember(member: ClusterMember): Promise<PlanningApplication>;
}
