import { useState, useCallback } from 'react';
import type { GeocodeResult, AuthorityId } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';

type CreateStep = 'postcode' | 'details';

interface CreateWatchZoneState {
  step: CreateStep;
  name: string;
  coordinates: GeocodeResult | null;
  radiusMetres: number;
  authorityId: AuthorityId | null;
  isSaving: boolean;
  error: string | null;
}

export function useCreateWatchZone(
  repository: WatchZoneRepository,
  navigate: (path: string) => void,
) {
  const [state, setState] = useState<CreateWatchZoneState>({
    step: 'postcode',
    name: '',
    coordinates: null,
    radiusMetres: 2000,
    authorityId: null,
    isSaving: false,
    error: null,
  });

  const setGeocode = useCallback((result: GeocodeResult) => {
    setState(prev => ({
      ...prev,
      coordinates: result,
      step: 'details',
      error: null,
    }));
  }, []);

  const setName = useCallback((name: string) => {
    setState(prev => ({ ...prev, name, error: null }));
  }, []);

  const setRadiusMetres = useCallback((radiusMetres: number) => {
    setState(prev => ({ ...prev, radiusMetres }));
  }, []);

  const setAuthorityId = useCallback((authorityId: AuthorityId) => {
    setState(prev => ({ ...prev, authorityId }));
  }, []);

  const save = useCallback(async () => {
    if (!state.coordinates) {
      setState(prev => ({ ...prev, error: 'Please look up a postcode first' }));
      return;
    }
    if (!state.name.trim()) {
      setState(prev => ({ ...prev, error: 'Please enter a name for this watch zone' }));
      return;
    }

    setState(prev => ({ ...prev, isSaving: true, error: null }));
    try {
      await repository.create({
        name: state.name.trim(),
        latitude: state.coordinates.latitude,
        longitude: state.coordinates.longitude,
        radiusMetres: state.radiusMetres,
        authorityId: state.authorityId as unknown as number,
      });
      navigate('/watch-zones');
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'An error occurred';
      setState(prev => ({ ...prev, isSaving: false, error: message }));
    }
  }, [state.coordinates, state.name, state.radiusMetres, state.authorityId, repository, navigate]);

  return {
    step: state.step,
    name: state.name,
    coordinates: state.coordinates,
    radiusMetres: state.radiusMetres,
    isSaving: state.isSaving,
    error: state.error,
    setGeocode,
    setName,
    setRadiusMetres,
    setAuthorityId,
    save,
  };
}
