import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { ApiDashboardAdapter } from './ApiDashboardAdapter';
import { DashboardPage } from './DashboardPage';

export function ConnectedDashboardPage() {
  const client = useApiClient();
  const port = useMemo(() => new ApiDashboardAdapter(client), [client]);

  return <DashboardPage port={port} />;
}
