import type { ApiClient } from './client';
import type {
  ApplicationsSort,
  ApplicationStatus,
  AuthorityListItem,
  MapCluster,
  PlanningApplication,
  PlanningApplicationSummary,
} from '../domain/types';

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

/**
 * Query for one page of the server-driven watch-zone applications list (GH#711,
 * Slice B). The server owns sort and status/unread filtering; the client follows
 * the opaque `X-Next-Cursor` header to exhaustion. `status` and `unread` are
 * mutually exclusive — sending both is a 400, so `unread` wins here defensively
 * (the calling hook already enforces single-select).
 */
export interface GetByZonePageOptions {
  readonly sort: ApplicationsSort;
  readonly status: ApplicationStatus | null;
  readonly unread: boolean;
  /** Opaque, sort-aware continuation token; `null`/omitted fetches page one. */
  readonly cursor: string | null;
  /** Page size; omitted lets the server apply its default (150, ceiling 500). */
  readonly limit?: number;
}

/** One page of list rows plus the cursor for the next page (`null` = last). */
export interface ZoneApplicationsPage {
  readonly rows: readonly PlanningApplicationSummary[];
  readonly nextCursor: string | null;
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
    /**
     * Server-driven, keyset-paginated page of a zone's applications (GH#711).
     * Drives `?sort/status/unread/cursor/limit` server-side and reads the next
     * cursor from the `X-Next-Cursor` response header (exposed cross-origin by
     * Slice A's CORS change). The body stays a bare array. The param-less
     * `getByZone` above is left untouched for backward-compatible callers.
     */
    getByZonePaged: (
      zoneId: string,
      options: GetByZonePageOptions,
    ): Promise<ZoneApplicationsPage> => {
      const params: Record<string, string> = { sort: options.sort };
      if (options.unread) {
        params.unread = 'true';
      } else if (options.status != null) {
        params.status = options.status;
      }
      if (options.cursor != null) {
        params.cursor = options.cursor;
      }
      if (options.limit != null) {
        params.limit = String(options.limit);
      }
      return client
        .getWithHeaders<readonly PlanningApplicationSummary[]>(
          `/v1/me/watch-zones/${zoneId}/applications`,
          params,
        )
        .then(({ body, headers }) => ({
          rows: body,
          nextCursor: headers.get('X-Next-Cursor'),
        }));
    },
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
