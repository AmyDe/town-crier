import type {
  ApplicationsBrowsePort,
  ApplicationsBrowseQuery,
  ApplicationsPageResult,
} from '../../../../domain/ports/applications-browse-port';

/**
 * Hand-written spy for the server-driven applications browse port. Records the
 * full query for each call (so tests can assert the sort/status/unread/cursor
 * sent to the server) and returns either a fixed single-page result or, for
 * multi-page / filter-aware tests, a caller-supplied responder.
 */
export class SpyApplicationsBrowsePort implements ApplicationsBrowsePort {
  fetchByZoneCalls: ApplicationsBrowseQuery[] = [];
  fetchByZoneResult: ApplicationsPageResult = { rows: [], nextCursor: null };
  fetchByZoneError: Error | null = null;
  fetchByZoneResponder:
    | ((query: ApplicationsBrowseQuery) => ApplicationsPageResult)
    | null = null;

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
}
