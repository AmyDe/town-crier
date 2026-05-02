import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { applicationsApi } from '../../api/applications';
import { designationsApi } from '../../api/designations';
import { savedApplicationsApi } from '../../api/savedApplications';
import type { ApplicationRepository } from '../../domain/ports/application-repository';
import type { DesignationRepository } from '../../domain/ports/designation-repository';
import type { SavedApplicationRepository } from '../../domain/ports/saved-application-repository';
import { ApplicationDetailPage } from './ApplicationDetailPage';

export function ConnectedApplicationDetailPage() {
  const client = useApiClient();

  const applicationRepository: ApplicationRepository = useMemo(
    () => ({
      fetchApplication: (uid) => applicationsApi(client).getByUid(uid),
    }),
    [client],
  );

  const designationRepository: DesignationRepository = useMemo(
    () => ({
      fetchDesignations: (latitude, longitude) =>
        designationsApi(client).getContext(latitude, longitude),
    }),
    [client],
  );

  const savedApplicationRepository: SavedApplicationRepository = useMemo(
    () => ({
      listSaved: () => savedApplicationsApi(client).list(),
      save: (application) => savedApplicationsApi(client).save(application),
      remove: (uid) => savedApplicationsApi(client).remove(uid),
    }),
    [client],
  );

  return (
    <ApplicationDetailPage
      applicationRepository={applicationRepository}
      designationRepository={designationRepository}
      savedApplicationRepository={savedApplicationRepository}
    />
  );
}
