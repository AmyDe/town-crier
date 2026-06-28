import type { ApiClient } from '../../api/client';
import type {
  ApplicationStatus,
  ClusterMember,
  MapCluster,
  PlanningApplication,
  WatchZoneId,
  WatchZoneSummary,
} from '../../domain/types';
import type { MapBounds, MapPort } from '../../domain/ports/map-port';
import { applicationsApi } from '../../api/applications';
import { watchZonesApi } from '../../api/watchZones';

export class ApiMapAdapter implements MapPort {
  private readonly apps: ReturnType<typeof applicationsApi>;
  private readonly zones: ReturnType<typeof watchZonesApi>;

  constructor(client: ApiClient) {
    this.apps = applicationsApi(client);
    this.zones = watchZonesApi(client);
  }

  async fetchMyZones(): Promise<readonly WatchZoneSummary[]> {
    return this.zones.list();
  }

  async fetchClusters(
    zoneId: WatchZoneId,
    bounds: MapBounds,
    zoom: number,
    status: ApplicationStatus | null,
  ): Promise<readonly MapCluster[]> {
    const bbox = `${bounds.west},${bounds.south},${bounds.east},${bounds.north}`;
    return this.apps.getClusters(zoneId as string, { bbox, zoom, status });
  }

  async fetchApplicationByMember(member: ClusterMember): Promise<PlanningApplication> {
    return this.apps.getByAuthorityAndName(member.authority, member.name);
  }
}
