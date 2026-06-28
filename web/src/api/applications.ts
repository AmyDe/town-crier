import type { ApiClient } from './client';
import type { AuthorityListItem, MapCluster, PlanningApplication } from '../domain/types';

interface UserApplicationAuthoritiesResponse {
  readonly authorities: readonly AuthorityListItem[];
  readonly count: number;
}

/** Wire shape for one cell of the map clusters endpoint (GH#698). */
interface MapClusterDto {
  readonly latitude: number;
  readonly longitude: number;
  readonly count: number;
  readonly statusCounts: Record<string, number>;
  readonly applicationId: { readonly authority: string; readonly name: string } | null;
}

export interface GetClustersOptions {
  /** `west,south,east,north` in WGS84 decimal degrees. */
  readonly bbox: string;
  /** Slippy-map zoom level (0..20). The server owns the zoom→grid mapping. */
  readonly zoom: number;
  /** Optional PlanIt `app_state` to filter by, server-side. */
  readonly status?: string | null;
}

function mapClusterToDomain(dto: MapClusterDto): MapCluster {
  return {
    latitude: dto.latitude,
    longitude: dto.longitude,
    count: dto.count,
    statusCounts: dto.statusCounts,
    member: dto.applicationId,
  };
}

export function applicationsApi(client: ApiClient) {
  return {
    getMyAuthorities: () =>
      client
        .get<UserApplicationAuthoritiesResponse>('/v1/me/application-authorities')
        .then((r) => r.authorities),
    getByZone: (zoneId: string) =>
      client.get<readonly PlanningApplication[]>(`/v1/me/watch-zones/${zoneId}/applications`),
    getByUid: (uid: string) =>
      client.get<PlanningApplication>(`/v1/applications/${uid}`),
    /**
     * Fetches the server-computed cluster aggregates for the visible viewport of
     * a single watch zone (GH#698). The map renders these instead of draining
     * every application, refetching on debounced pan/zoom.
     */
    getClusters: (zoneId: string, options: GetClustersOptions) => {
      const params: Record<string, string> = {
        bbox: options.bbox,
        zoom: String(options.zoom),
      };
      if (options.status != null) {
        params.status = options.status;
      }
      return client
        .get<readonly MapClusterDto[]>(
          `/v1/me/watch-zones/${zoneId}/applications/clusters`,
          params,
        )
        .then((dtos) => dtos.map(mapClusterToDomain));
    },
    /**
     * Composite-key point-read of a single application via its `{authority, name}`
     * identity (mirrors iOS PR #700). `name` is a path-wildcard segment that may
     * contain slashes (e.g. "22/1234/FUL"), so it is interpolated raw to match
     * the server's greedy `{name...}` route. The returned record carries `uid`
     * for navigating to the existing `/applications/{uid}` detail route.
     */
    getByAuthorityAndName: (authority: string, name: string) =>
      client.get<PlanningApplication>(`/v1/applications/${authority}/${name}`),
  };
}
