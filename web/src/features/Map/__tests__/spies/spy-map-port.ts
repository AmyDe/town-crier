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
  holdClusterRequests = false;
  private pendingClusterRequests: {
    resolve: (clusters: readonly MapCluster[]) => void;
    reject: (error: Error) => void;
  }[] = [];

  get pendingClusterRequestCount(): number {
    return this.pendingClusterRequests.length;
  }

  /** Settles the oldest held request, so tests control the order requests finish in. */
  settleClusterRequest(outcome: readonly MapCluster[] | Error, index = 0): void {
    const [pending] = this.pendingClusterRequests.splice(index, 1);
    if (!pending) throw new Error('no held cluster request');
    if (outcome instanceof Error) pending.reject(outcome);
    else pending.resolve(outcome);
  }

  async fetchClusters(
    zoneId: WatchZoneId,
    bounds: MapBounds,
    zoom: number,
    status: ApplicationStatus | null,
  ): Promise<readonly MapCluster[]> {
    this.fetchClustersCalls.push({ zoneId, bounds, zoom, status });
    if (this.holdClusterRequests) {
      return new Promise((resolve, reject) => {
        this.pendingClusterRequests.push({ resolve, reject });
      });
    }
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
