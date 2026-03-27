import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useCreateWatchZone } from '../useCreateWatchZone';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { aWatchZone } from './fixtures/watch-zone.fixtures';
import { asAuthorityId } from '../../../domain/types';

describe('useCreateWatchZone', () => {
  let spy: SpyWatchZoneRepository;
  let navigatedTo: string | null;

  beforeEach(() => {
    spy = new SpyWatchZoneRepository();
    navigatedTo = null;
  });

  function navigate(path: string) {
    navigatedTo = path;
  }

  it('starts in initial step with empty state', () => {
    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    expect(result.current.step).toBe('postcode');
    expect(result.current.name).toBe('');
    expect(result.current.coordinates).toBeNull();
    expect(result.current.radiusMetres).toBe(2000);
    expect(result.current.isSaving).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('advances to radius step after geocode', () => {
    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    act(() => {
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 });
    });

    expect(result.current.step).toBe('details');
    expect(result.current.coordinates).toEqual({ latitude: 52.2053, longitude: 0.1218 });
  });

  it('allows setting name and radius', () => {
    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    act(() => {
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 });
    });

    act(() => {
      result.current.setName('Home');
      result.current.setRadiusMetres(5000);
    });

    expect(result.current.name).toBe('Home');
    expect(result.current.radiusMetres).toBe(5000);
  });

  it('saves zone and navigates to list', async () => {
    spy.createResult = aWatchZone();

    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    act(() => {
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 });
    });

    act(() => {
      result.current.setName('Home');
      result.current.setAuthorityId(asAuthorityId(1));
    });

    await act(async () => {
      await result.current.save();
    });

    expect(spy.createCalls).toHaveLength(1);
    expect(spy.createCalls[0]).toEqual({
      name: 'Home',
      latitude: 52.2053,
      longitude: 0.1218,
      radiusMetres: 2000,
      authorityId: 1,
    });
    expect(navigatedTo).toBe('/watch-zones');
  });

  it('sets error when save fails', async () => {
    spy.createError = new Error('Create failed');

    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    act(() => {
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 });
      result.current.setName('Home');
      result.current.setAuthorityId(asAuthorityId(1));
    });

    await act(async () => {
      await result.current.save();
    });

    expect(result.current.error).toBe('Create failed');
    expect(navigatedTo).toBeNull();
  });

  it('does not save without coordinates', async () => {
    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    act(() => {
      result.current.setName('Home');
    });

    await act(async () => {
      await result.current.save();
    });

    expect(spy.createCalls).toHaveLength(0);
    expect(result.current.error).toBe('Please look up a postcode first');
  });

  it('does not save without a name', async () => {
    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    act(() => {
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 });
      result.current.setAuthorityId(asAuthorityId(1));
    });

    await act(async () => {
      await result.current.save();
    });

    expect(spy.createCalls).toHaveLength(0);
    expect(result.current.error).toBe('Please enter a name for this watch zone');
  });
});
