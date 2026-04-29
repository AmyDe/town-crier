import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { savedApplicationsApi } from '../../api/savedApplications';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
import { SavedApplicationsPage } from './SavedApplicationsPage';

export function ConnectedSavedApplicationsPage() {
  const client = useApiClient();

  const savedRepository: SavedApplicationRepository = useMemo(
    () => ({
      listSaved: () => savedApplicationsApi(client).list(),
      save: (uid) => savedApplicationsApi(client).save(uid),
      remove: (uid) => savedApplicationsApi(client).remove(uid),
    }),
    [client],
  );

  return <SavedApplicationsPage savedRepository={savedRepository} />;
}
