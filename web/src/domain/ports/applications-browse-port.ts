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
  /**
   * Returns the zone's **full** unread total, independent of the main list's
   * pagination (GH#716, Problem 2). Sourced from the unread-only query exhausted
   * across every page, so loading more main-list pages never changes it. This is
   * what the "Unread (N)" chip counts — a whole-zone signal, not the rows loaded
   * so far. Deliberately zone-scoped (not the account-wide notification-state
   * snapshot, which would over-count a multi-zone user).
   */
  countUnread(zoneId: WatchZoneId): Promise<number>;
}
