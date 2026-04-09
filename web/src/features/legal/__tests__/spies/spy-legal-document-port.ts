import type { LegalDocument } from '../../../../domain/types';
import type { LegalDocumentPort } from '../../../../domain/ports/legal-document-port';

export class SpyLegalDocumentPort implements LegalDocumentPort {
  fetchDocumentCalls: string[] = [];
  fetchDocumentResult: LegalDocument = {
    documentType: 'privacy',
    title: 'Privacy Policy',
    lastUpdated: '2026-03-16',
    sections: [],
  };
  fetchDocumentError: Error | null = null;

  async fetchDocument(documentType: string): Promise<LegalDocument> {
    this.fetchDocumentCalls.push(documentType);
    if (this.fetchDocumentError) {
      throw this.fetchDocumentError;
    }
    return this.fetchDocumentResult;
  }
}
