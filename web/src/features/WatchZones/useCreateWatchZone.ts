import { useState, useCallback } from 'react';
import type { GeocodeResult } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

type CreateStep = 'postcode' | 'details' | 'confirm';

interface CreateWatchZoneState {
  step: CreateStep;
  name: string;
  postcode: string;
  coordinates: GeocodeResult | null;
  radiusMetres: number;
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
    postcode: '',
    coordinates: null,
    radiusMetres: 2000,
    isSaving: false,
    error: null,
  });

  const setGeocode = useCallback((result: GeocodeResult, enteredPostcode: string) => {
    setState(prev => ({
      ...prev,
      coordinates: result,
      postcode: enteredPostcode,
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

  const confirmDetails = useCallback(() => {
    if (!state.name.trim()) {
      setState(prev => ({ ...prev, error: 'Please enter a name for this watch zone' }));
      return;
    }
    setState(prev => ({ ...prev, step: 'confirm', error: null }));
  }, [state.name]);

  const save = useCallback(async () => {
    if (!state.coordinates) {
      setState(prev => ({ ...prev, error: 'Please look up a postcode first' }));
      return;
    }

    setState(prev => ({ ...prev, isSaving: true, error: null }));
    try {
      await repository.create({
        name: state.name.trim(),
        latitude: state.coordinates.latitude,
        longitude: state.coordinates.longitude,
        radiusMetres: state.radiusMetres,
      });
      navigate('/watch-zones');
    } catch (err: unknown) {
      const message = extractErrorMessage(err);
      setState(prev => ({ ...prev, isSaving: false, error: message }));
    }
  }, [state.coordinates, state.name, state.radiusMetres, repository, navigate]);

  return {
    step: state.step,
    name: state.name,
    postcode: state.postcode,
    coordinates: state.coordinates,
    radiusMetres: state.radiusMetres,
    isSaving: state.isSaving,
    error: state.error,
    setGeocode,
    setName,
    setRadiusMetres,
    confirmDetails,
    save,
  };
}
