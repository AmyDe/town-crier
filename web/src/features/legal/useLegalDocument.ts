import { useState, useEffect } from 'react';
import type { LegalDocument } from '../../domain/types';
import type { LegalDocumentPort } from '../../domain/ports/legal-document-port';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

interface LegalDocumentState {
  document: LegalDocument | null;
  isLoading: boolean;
  error: string | null;
}

const initialState: LegalDocumentState = {
  document: null,
  isLoading: true,
  error: null,
};

export function useLegalDocument(port: LegalDocumentPort, documentType: string) {
  const [state, setState] = useState<LegalDocumentState>(initialState);

  useEffect(() => {
    let cancelled = false;
    setState(initialState);

    port.fetchDocument(documentType).then(
      (doc) => {
        if (!cancelled) setState({ document: doc, isLoading: false, error: null });
      },
      (err: unknown) => {
        if (!cancelled) {
          const message = extractErrorMessage(err, 'Failed to load document');
          setState({ document: null, isLoading: false, error: message });
        }
      },
    );

    return () => { cancelled = true; };
  }, [port, documentType]);

  return state;
}
