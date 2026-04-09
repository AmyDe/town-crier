import type { LegalDocument } from '../../domain/types';
import type { LegalDocumentPort } from '../../domain/ports/legal-document-port';

export class ApiLegalDocumentPort implements LegalDocumentPort {
  private readonly baseUrl: string;
  private readonly fetchFn: typeof globalThis.fetch;

  constructor(baseUrl: string, fetchFn: typeof globalThis.fetch = globalThis.fetch.bind(globalThis)) {
    this.baseUrl = baseUrl;
    this.fetchFn = fetchFn;
  }

  async fetchDocument(documentType: string): Promise<LegalDocument> {
    const response = await this.fetchFn(`${this.baseUrl}/v1/legal/${documentType}`);

    if (!response.ok) {
      throw new Error('Failed to load legal document');
    }

    return response.json() as Promise<LegalDocument>;
  }
}
