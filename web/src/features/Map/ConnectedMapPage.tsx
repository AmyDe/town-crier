import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { ApiMapAdapter } from './ApiMapAdapter';
import { MapPage } from './MapPage';

export function ConnectedMapPage() {
  const client = useApiClient();
  const port = useMemo(() => new ApiMapAdapter(client), [client]);

  return <MapPage port={port} />;
}
