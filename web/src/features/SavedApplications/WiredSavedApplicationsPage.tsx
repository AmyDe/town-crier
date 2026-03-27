import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { ApiSavedApplicationRepository } from './ApiSavedApplicationRepository';
import { SavedApplicationsPage } from './SavedApplicationsPage';

export function WiredSavedApplicationsPage() {
  const client = useApiClient();
  const repository = useMemo(() => new ApiSavedApplicationRepository(client), [client]);

  return <SavedApplicationsPage repository={repository} />;
}
