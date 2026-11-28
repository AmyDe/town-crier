import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { savedApplicationsApi } from '../../api/savedApplications';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
import { SavedApplicationsPage } from './SavedApplicationsPage';

export function ConnectedSavedApplicationsPage() {
  const client = useApiClient();

  const savedRepository: SavedApplicationRepository = useMemo(
    () => {
      const api = savedApplicationsApi(client);
      return {
        listSaved: api.list,
        save: api.save,
        remove: api.remove,
      };
    },
    [client],
  );

  return <SavedApplicationsPage savedRepository={savedRepository} />;
}
