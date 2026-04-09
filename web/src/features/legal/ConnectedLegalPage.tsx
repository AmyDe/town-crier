import { useMemo } from 'react';
import { ApiLegalDocumentPort } from './ApiLegalDocumentPort';
import { LegalPage } from './LegalPage';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL as string || 'http://localhost:5000';

export function ConnectedLegalPage() {
  const port = useMemo(() => new ApiLegalDocumentPort(API_BASE_URL), []);

  return <LegalPage port={port} />;
}
