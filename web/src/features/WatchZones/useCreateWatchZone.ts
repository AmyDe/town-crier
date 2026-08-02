import { useState, useCallback } from 'react';
import type { GeocodeResult, WatchZoneBoundary } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import type { ShapeMode } from './ShapeModeToggle';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

type CreateStep = 'postcode' | 'details' | 'confirm';

const MISSING_BOUNDARY_ERROR =
  'Draw a shape with at least 3 points, then close it by clicking the first point again.';

interface CreateWatchZoneState {
  step: CreateStep;
  name: string;
  postcode: string;
  coordinates: GeocodeResult | null;
  radiusMetres: number;
  shapeMode: ShapeMode;
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
    shapeMode: 'circle',
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

  const setShapeMode = useCallback((shapeMode: ShapeMode) => {
    setState(prev => ({ ...prev, shapeMode, error: null }));
  }, []);

  const confirmDetails = useCallback(
    (boundary?: WatchZoneBoundary | null) => {
      if (!state.name.trim()) {
        setState(prev => ({ ...prev, error: 'Please enter a name for this watch zone' }));
        return;
      }
      if (state.shapeMode === 'custom' && !boundary) {
        setState(prev => ({ ...prev, error: MISSING_BOUNDARY_ERROR }));
        return;
      }
      setState(prev => ({ ...prev, step: 'confirm', error: null }));
    },
    [state.name, state.shapeMode],
  );

  const save = useCallback(
    async (boundary?: WatchZoneBoundary | null) => {
      if (!state.coordinates) {
        setState(prev => ({ ...prev, error: 'Please look up a postcode first' }));
        return;
      }
      if (state.shapeMode === 'custom' && !boundary) {
        setState(prev => ({ ...prev, error: MISSING_BOUNDARY_ERROR }));
        return;
      }

      setState(prev => ({ ...prev, isSaving: true, error: null }));
      try {
        await repository.create({
          name: state.name.trim(),
          latitude: state.coordinates.latitude,
          longitude: state.coordinates.longitude,
          radiusMetres: state.radiusMetres,
          ...(state.shapeMode === 'custom' && boundary ? { boundary } : {}),
        });
        navigate('/watch-zones');
      } catch (err: unknown) {
        const message = extractErrorMessage(err);
        setState(prev => ({ ...prev, isSaving: false, error: message }));
      }
    },
    [state.coordinates, state.name, state.radiusMetres, state.shapeMode, repository, navigate],
  );

  return {
    step: state.step,
    name: state.name,
    postcode: state.postcode,
    coordinates: state.coordinates,
    radiusMetres: state.radiusMetres,
    shapeMode: state.shapeMode,
    isSaving: state.isSaving,
    error: state.error,
    setGeocode,
    setName,
    setRadiusMetres,
    setShapeMode,
    confirmDetails,
    save,
  };
}
