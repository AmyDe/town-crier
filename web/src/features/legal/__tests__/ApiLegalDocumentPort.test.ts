import { describe, it, expect, beforeEach } from 'vitest';
import { ApiLegalDocumentPort } from '../ApiLegalDocumentPort';
import type { LegalDocument } from '../../../domain/types';

class StubFetch {
  lastUrl: string | null = null;
  responseBody: unknown = {};
  responseStatus = 200;
  shouldReject = false;
  rejectError: Error = new Error('Network failure');

  readonly fetch: typeof globalThis.fetch = async (input: RequestInfo | URL) => {
    this.lastUrl = String(input);
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

describe('ApiLegalDocumentPort', () => {
  let stub: StubFetch;
  let port: ApiLegalDocumentPort;
  const baseUrl = 'https://api.example.com';

  beforeEach(() => {
    stub = new StubFetch();
    port = new ApiLegalDocumentPort(baseUrl, stub.fetch);
  });

  it('fetches from the correct URL for privacy', async () => {
    const apiResponse = {
      documentType: 'privacy',
      title: 'Privacy Policy',
      lastUpdated: '2026-03-16',
      sections: [{ heading: 'Section 1', body: 'Body text' }],
    };
    stub.responseBody = apiResponse;

    const result = await port.fetchDocument('privacy');

    expect(stub.lastUrl).toBe('https://api.example.com/v1/legal/privacy');
    expect(result).toEqual<LegalDocument>(apiResponse);
  });

  it('fetches from the correct URL for terms', async () => {
    stub.responseBody = {
      documentType: 'terms',
      title: 'Terms of Service',
      lastUpdated: '2026-03-16',
      sections: [],
    };

    await port.fetchDocument('terms');

    expect(stub.lastUrl).toBe('https://api.example.com/v1/legal/terms');
  });

  it('throws an error when the API returns a non-ok status', async () => {
    stub.responseStatus = 404;

    await expect(port.fetchDocument('unknown')).rejects.toThrow(
      'Failed to load legal document',
    );
  });

  it('throws an error when fetch rejects', async () => {
    stub.shouldReject = true;
    stub.rejectError = new Error('Network failure');

    await expect(port.fetchDocument('privacy')).rejects.toThrow('Network failure');
  });
});
