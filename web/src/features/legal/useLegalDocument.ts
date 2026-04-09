import { useState, useEffect, useCallback } from 'react';
import type { LegalDocument } from '../../domain/types';
import type { LegalDocumentPort } from '../../domain/ports/legal-document-port';

interface LegalDocumentState {
  document: LegalDocument | null;
  isLoading: boolean;
  error: string | null;
}

export function useLegalDocument(port: LegalDocumentPort, documentType: string) {
  const [state, setState] = useState<LegalDocumentState>({
    document: null,
    isLoading: true,
    error: null,
  });

  const loadDocument = useCallback(async () => {
    setState({ document: null, isLoading: true, error: null });
    try {
      const doc = await port.fetchDocument(documentType);
      setState({ document: doc, isLoading: false, error: null });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to load document';
      setState({ document: null, isLoading: false, error: message });
    }
  }, [port, documentType]);

  useEffect(() => {
    loadDocument();
  }, [loadDocument]);

  return state;
}
