import type { LegalDocument } from '../types';

export interface LegalDocumentPort {
  fetchDocument(documentType: string): Promise<LegalDocument>;
}
