import type {
  ApplicationsBrowsePort,
  ApplicationsBrowseQuery,
  ApplicationsPageResult,
} from '../../../../domain/ports/applications-browse-port';
import type { WatchZoneId } from '../../../../domain/types';

/** `unreadTotal` is decoupled from `fetchByZone` pages so tests can prove the chip count is whole-zone. */
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
