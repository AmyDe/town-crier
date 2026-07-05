import { describe, it, expect, beforeEach } from 'vitest';
import { ApiSearchPort } from '../ApiSearchPort';

class StubFetch {
  lastUrl: string | null = null;
  lastInit: RequestInit | undefined = undefined;
  responseBody: unknown = { query: '', results: [], refineQuery: false };
  responseStatus = 200;
  shouldReject = false;
  rejectError: Error = new Error('Network failure');

  readonly fetch: typeof globalThis.fetch = async (
    input: RequestInfo | URL,
    init?: RequestInit,
  ) => {
    this.lastUrl = String(input);
    this.lastInit = init;
    if (this.shouldReject) {
      throw this.rejectError;
    }
    return {
      ok: this.responseStatus >= 200 && this.responseStatus < 300,
      status: this.responseStatus,
      json: async () => this.responseBody,
    } as Response;
  };
}

describe('ApiSearchPort', () => {
  let stub: StubFetch;
  let port: ApiSearchPort;
  const baseUrl = 'https://api.example.com';

  beforeEach(() => {
    stub = new StubFetch();
    port = new ApiSearchPort(baseUrl, stub.fetch);
  });

  it('fetches from the search endpoint with the query string', async () => {
    await port.search('mill road', null);

    expect(stub.lastUrl).toBe('https://api.example.com/v1/applications/search?q=mill+road');
  });

  it('adds the authority filter when provided', async () => {
    await port.search('mill road', 'cambridge');

    const url = new URL(stub.lastUrl!);
    expect(url.searchParams.get('q')).toBe('mill road');
    expect(url.searchParams.get('authority')).toBe('cambridge');
  });

  it('never attaches an Authorization header — this endpoint is anonymous', async () => {
    await port.search('mill road', null);

    const headers = stub.lastInit?.headers;
    expect(headers).toBeUndefined();
  });

  it('maps a successful response to results and refineQuery', async () => {
    stub.responseBody = {
      query: 'mill road',
      results: [
        {
          reference: '22/1234/FUL',
          authoritySlug: 'cambridge',
          authorityName: 'Cambridge City Council',
          address: '12 Mill Road, Cambridge, CB1 2AD',
          appState: 'Permitted',
          startDate: '2026-01-15',
          decidedDate: '2026-03-01',
        },
      ],
      refineQuery: true,
    };

    const outcome = await port.search('mill road', null);

    expect(outcome.refineQuery).toBe(true);
    expect(outcome.results).toEqual([
      {
        reference: '22/1234/FUL',
        authoritySlug: 'cambridge',
        authorityName: 'Cambridge City Council',
        address: '12 Mill Road, Cambridge, CB1 2AD',
        appState: 'Permitted',
        startDate: '2026-01-15',
        decidedDate: '2026-03-01',
      },
    ]);
  });

  it('maps nullable fields through as null', async () => {
    stub.responseBody = {
      query: 'ref',
      results: [
        {
          reference: '24/0001/FUL',
          authoritySlug: 'adur',
          authorityName: 'Adur District Council',
          address: '1 High Street',
          appState: null,
          startDate: null,
          decidedDate: null,
        },
      ],
      refineQuery: false,
    };

    const outcome = await port.search('ref', null);

    expect(outcome.results[0]).toEqual({
      reference: '24/0001/FUL',
      authoritySlug: 'adur',
      authorityName: 'Adur District Council',
      address: '1 High Street',
      appState: null,
      startDate: null,
      decidedDate: null,
    });
  });

  it('throws when the API returns a non-ok status', async () => {
    stub.responseStatus = 400;

    await expect(port.search('a', null)).rejects.toThrow('Failed to search applications');
  });

  it('throws when fetch rejects', async () => {
    stub.shouldReject = true;
    stub.rejectError = new Error('Network failure');

    await expect(port.search('mill road', null)).rejects.toThrow('Network failure');
  });
});
