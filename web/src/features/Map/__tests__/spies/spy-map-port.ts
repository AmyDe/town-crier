import type {
  ApplicationStatus,
  ClusterMember,
  MapCluster,
  PlanningApplication,
  WatchZoneId,
  WatchZoneSummary,
} from '../../../../domain/types';
import type { MapBounds, MapPort } from '../../../../domain/ports/map-port';
import { anApplication } from '../fixtures/map.fixtures';

export interface FetchClustersCall {
  readonly zoneId: WatchZoneId;
  readonly bounds: MapBounds;
  readonly zoom: number;
  readonly status: ApplicationStatus | null;
}

export class SpyMapPort implements MapPort {
  fetchMyZonesCalls = 0;
  fetchMyZonesResult: readonly WatchZoneSummary[] = [];
  fetchMyZonesError: Error | null = null;

  async fetchMyZones(): Promise<readonly WatchZoneSummary[]> {
    this.fetchMyZonesCalls++;
    if (this.fetchMyZonesError) {
      throw this.fetchMyZonesError;
    }
    return this.fetchMyZonesResult;
  }

  fetchClustersCalls: FetchClustersCall[] = [];
  fetchClustersResult: readonly MapCluster[] = [];
  /** Per-zone overrides; falls back to `fetchClustersResult` when absent. */
  fetchClustersResultsByZone: Map<string, readonly MapCluster[]> = new Map();
  fetchClustersError: Error | null = null;

  async fetchClusters(
    zoneId: WatchZoneId,
    bounds: MapBounds,
    zoom: number,
    status: ApplicationStatus | null,
  ): Promise<readonly MapCluster[]> {
    this.fetchClustersCalls.push({ zoneId, bounds, zoom, status });
    if (this.fetchClustersError) {
      throw this.fetchClustersError;
    }
    return this.fetchClustersResultsByZone.get(zoneId as string) ?? this.fetchClustersResult;
  }

  fetchApplicationByMemberCalls: ClusterMember[] = [];
  fetchApplicationByMemberResult: PlanningApplication = anApplication();
  fetchApplicationByMemberError: Error | null = null;

  async fetchApplicationByMember(member: ClusterMember): Promise<PlanningApplication> {
    this.fetchApplicationByMemberCalls.push(member);
    if (this.fetchApplicationByMemberError) {
      throw this.fetchApplicationByMemberError;
    }
    return this.fetchApplicationByMemberResult;
  }
}
