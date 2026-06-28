import type {
  ApplicationsBrowsePort,
  ApplicationsBrowseQuery,
  ApplicationsPageResult,
} from '../../../../domain/ports/applications-browse-port';
import type { WatchZoneId } from '../../../../domain/types';

/**
 * Hand-written spy for the server-driven applications browse port. Records the
 * full query for each call (so tests can assert the sort/status/unread/cursor
 * sent to the server) and returns either a fixed single-page result or, for
 * multi-page / filter-aware tests, a caller-supplied responder.
 *
 * `countUnread` is modelled by a settable whole-zone total (`unreadTotal`),
 * decoupled from `fetchByZone`'s pages so a test can return a multi-page list
 * yet a larger separate unread total — proving the chip count is whole-zone, not
 * loaded-rows derived.
 */
export class SpyApplicationsBrowsePort implements ApplicationsBrowsePort {
  fetchByZoneCalls: ApplicationsBrowseQuery[] = [];
  fetchByZoneResult: ApplicationsPageResult = { rows: [], nextCursor: null };
  fetchByZoneError: Error | null = null;
  fetchByZoneResponder:
    | ((query: ApplicationsBrowseQuery) => ApplicationsPageResult)
    | null = null;

  /** Whole-zone unread total returned by `countUnread`. */
  unreadTotal = 0;
  countUnreadCalls: WatchZoneId[] = [];
  countUnreadError: Error | null = null;

  async fetchByZone(query: ApplicationsBrowseQuery): Promise<ApplicationsPageResult> {
    this.fetchByZoneCalls.push(query);
    if (this.fetchByZoneResponder) {
      return this.fetchByZoneResponder(query);
    }
    if (this.fetchByZoneError) {
      throw this.fetchByZoneError;
    }
    return this.fetchByZoneResult;
  }

  async countUnread(zoneId: WatchZoneId): Promise<number> {
    this.countUnreadCalls.push(zoneId);
    if (this.countUnreadError) {
      throw this.countUnreadError;
    }
    return this.unreadTotal;
  }
}
