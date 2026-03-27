import { useState, useCallback } from 'react';
import type { GeocodeResult, AuthorityListItem, GroupDetail } from '../../domain/types';
import type { GroupsRepository } from '../../domain/ports/groups-repository';

type CreateStep = 'postcode' | 'radius' | 'authority' | 'name';

interface GroupCreateState {
  step: CreateStep;
  location: GeocodeResult | null;
  radius: number | null;
  authority: AuthorityListItem | null;
  isSubmitting: boolean;
  error: string | null;
}

export function useGroupCreate(repository: GroupsRepository) {
  const [state, setState] = useState<GroupCreateState>({
    step: 'postcode',
    location: null,
    radius: null,
    authority: null,
    isSubmitting: false,
    error: null,
  });

  const setLocation = useCallback((location: GeocodeResult) => {
    setState((prev) => ({ ...prev, location, step: 'radius' }));
  }, []);

  const setRadius = useCallback((radius: number) => {
    setState((prev) => ({ ...prev, radius, step: 'authority' }));
  }, []);

  const setAuthority = useCallback((authority: AuthorityListItem) => {
    setState((prev) => ({ ...prev, authority, step: 'name' }));
  }, []);

  const submit = useCallback(
    async (name: string): Promise<GroupDetail | null> => {
      if (!state.location || state.radius === null || !state.authority) {
        return null;
      }

      setState((prev) => ({ ...prev, isSubmitting: true, error: null }));
      try {
        const result = await repository.createGroup({
          name,
          latitude: state.location.latitude,
          longitude: state.location.longitude,
          radiusMetres: state.radius,
          authorityId: state.authority.id,
        });
        setState((prev) => ({ ...prev, isSubmitting: false }));
        return result;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to create group';
        setState((prev) => ({ ...prev, isSubmitting: false, error: message }));
        return null;
      }
    },
    [repository, state.location, state.radius, state.authority],
  );

  return {
    ...state,
    setLocation,
    setRadius,
    setAuthority,
    submit,
  };
}
