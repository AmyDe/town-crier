import type {
  WatchZoneId,
  PlanningApplicationSummary,
  ApplicationStatus,
  ApplicationsSort,
} from '../types';

/**
 * One page request for a watch zone's applications, driving sort and the
 * status/unread filter **server-side** (GH#711, Slice B). `status` and `unread`
 * are mutually exclusive (the caller enforces single-select). `cursor` is the
 * opaque, sort-aware continuation token — `null` fetches the first page.
 */
export interface ApplicationsBrowseQuery {
  readonly zoneId: WatchZoneId;
  readonly sort: ApplicationsSort;
  readonly status: ApplicationStatus | null;
  readonly unread: boolean;
  readonly cursor: string | null;
}

/**
 * One page of list rows plus the cursor for the following page. `nextCursor` is
 * `null` on the last page (the `X-Next-Cursor` header was absent), which is the
 * signal to stop paging.
 */
export interface ApplicationsPageResult {
  readonly rows: readonly PlanningApplicationSummary[];
  readonly nextCursor: string | null;
}

export interface ApplicationsBrowsePort {
  fetchByZone(query: ApplicationsBrowseQuery): Promise<ApplicationsPageResult>;
}
