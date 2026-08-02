import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useCreateWatchZone } from '../useCreateWatchZone';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { aWatchZone, aWatchZoneBoundary } from './fixtures/watch-zone.fixtures';

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
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
    });

    expect(result.current.step).toBe('details');
    expect(result.current.coordinates).toEqual({ latitude: 52.2053, longitude: 0.1218 });
    expect(result.current.postcode).toBe('CB1 2AD');
  });

  it('allows setting name and radius', () => {
    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    act(() => {
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
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
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
    });

    act(() => {
      result.current.setName('Home');
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
    });
    expect(navigatedTo).toBe('/watch-zones');
  });

  it('sets error when save fails', async () => {
    spy.createError = new Error('Create failed');

    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    act(() => {
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
      result.current.setName('Home');
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

  it('does not advance to confirm without a name', () => {
    const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

    act(() => {
      result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
    });

    act(() => {
      result.current.confirmDetails();
    });

    expect(result.current.step).toBe('details');
    expect(result.current.error).toBe('Please enter a name for this watch zone');
  });

  describe('shape mode', () => {
    it('starts in circle shape mode', () => {
      const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

      expect(result.current.shapeMode).toBe('circle');
    });

    it('allows switching to custom shape mode', () => {
      const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

      act(() => {
        result.current.setShapeMode('custom');
      });

      expect(result.current.shapeMode).toBe('custom');
    });

    it('does not advance to confirm in custom mode without a closed boundary', () => {
      const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

      act(() => {
        result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
        result.current.setName('Home');
        result.current.setShapeMode('custom');
      });

      act(() => {
        result.current.confirmDetails(null);
      });

      expect(result.current.step).toBe('details');
      expect(result.current.error).toBe(
        'Draw a shape with at least 3 points, then close it by clicking the first point again.',
      );
    });

    it('advances to confirm in custom mode once a boundary is provided', () => {
      const { result } = renderHook(() => useCreateWatchZone(spy, navigate));
      const boundary = aWatchZoneBoundary();

      act(() => {
        result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
        result.current.setName('Home');
        result.current.setShapeMode('custom');
      });

      act(() => {
        result.current.confirmDetails(boundary);
      });

      expect(result.current.step).toBe('confirm');
      expect(result.current.error).toBeNull();
    });

    it('includes the boundary in the create request in custom mode', async () => {
      const boundary = aWatchZoneBoundary();
      spy.createResult = aWatchZone({ boundary });

      const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

      act(() => {
        result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
        result.current.setName('Home');
        result.current.setShapeMode('custom');
      });

      await act(async () => {
        await result.current.save(boundary);
      });

      expect(spy.createCalls).toHaveLength(1);
      expect(spy.createCalls[0]).toEqual({
        name: 'Home',
        latitude: 52.2053,
        longitude: 0.1218,
        radiusMetres: 2000,
        boundary,
      });
      expect(navigatedTo).toBe('/watch-zones');
    });

    it('omits the boundary from the create request while in circle mode', async () => {
      spy.createResult = aWatchZone();
      const boundary = aWatchZoneBoundary();

      const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

      act(() => {
        result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
        result.current.setName('Home');
      });

      await act(async () => {
        // Even if a stale boundary value is passed in, circle mode ignores it.
        await result.current.save(boundary);
      });

      const call = spy.createCalls[0] as Record<string, unknown>;
      expect('boundary' in call).toBe(false);
    });

    it('does not save a custom-mode zone without a boundary', async () => {
      const { result } = renderHook(() => useCreateWatchZone(spy, navigate));

      act(() => {
        result.current.setGeocode({ latitude: 52.2053, longitude: 0.1218 }, 'CB1 2AD');
        result.current.setName('Home');
        result.current.setShapeMode('custom');
      });

      await act(async () => {
        await result.current.save(null);
      });

      expect(spy.createCalls).toHaveLength(0);
      expect(result.current.error).toBe(
        'Draw a shape with at least 3 points, then close it by clicking the first point again.',
      );
    });
  });
});
