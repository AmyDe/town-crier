import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { useZoneEdit } from '../useZoneEdit';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { aWatchZone, aWatchZoneBoundary } from './fixtures/watch-zone.fixtures';

describe('useZoneEdit', () => {
  let spy: SpyWatchZoneRepository;

  beforeEach(() => {
    spy = new SpyWatchZoneRepository();
  });

  it('initialises name and radius from the zone prop', () => {
    const zone = aWatchZone({ name: 'Office', radiusMetres: 5000 });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    expect(result.current.name).toBe('Office');
    expect(result.current.radiusMetres).toBe(5000);
    expect(result.current.isDirty).toBe(false);
  });

  it('tracks name changes and marks dirty', () => {
    const zone = aWatchZone();

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setName('New Name');
    });

    expect(result.current.name).toBe('New Name');
    expect(result.current.isDirty).toBe(true);
  });

  it('tracks radius changes and marks dirty', () => {
    const zone = aWatchZone({ radiusMetres: 2000 });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setRadiusMetres(5000);
    });

    expect(result.current.radiusMetres).toBe(5000);
    expect(result.current.isDirty).toBe(true);
  });

  it('is not dirty when values match the original zone', () => {
    const zone = aWatchZone({ name: 'Home', radiusMetres: 2000 });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setName('Changed');
    });

    expect(result.current.isDirty).toBe(true);

    act(() => {
      result.current.setName('Home');
    });

    expect(result.current.isDirty).toBe(false);
  });

  it('calls updateZone with changed fields on save', async () => {
    const zone = aWatchZone({ name: 'Home', radiusMetres: 2000 });
    spy.updateZoneResult = aWatchZone({ name: 'Office', radiusMetres: 2000 });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setName('Office');
    });

    await act(async () => {
      await result.current.save();
    });

    expect(spy.updateZoneCalls).toHaveLength(1);
    expect(spy.updateZoneCalls[0]?.zoneId).toBe(zone.id);
    expect(spy.updateZoneCalls[0]?.data).toEqual({ name: 'Office' });
  });

  it('sends only changed fields in the patch', async () => {
    const zone = aWatchZone({ name: 'Home', radiusMetres: 2000 });
    spy.updateZoneResult = aWatchZone({ name: 'Home', radiusMetres: 5000 });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setRadiusMetres(5000);
    });

    await act(async () => {
      await result.current.save();
    });

    expect(spy.updateZoneCalls[0]?.data).toEqual({ radiusMetres: 5000 });
  });

  it('sends both fields when both changed', async () => {
    const zone = aWatchZone({ name: 'Home', radiusMetres: 2000 });
    spy.updateZoneResult = aWatchZone({ name: 'Office', radiusMetres: 5000 });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setName('Office');
      result.current.setRadiusMetres(5000);
    });

    await act(async () => {
      await result.current.save();
    });

    expect(spy.updateZoneCalls[0]?.data).toEqual({ name: 'Office', radiusMetres: 5000 });
  });

  it('does not call updateZone when not dirty', async () => {
    const zone = aWatchZone();

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    await act(async () => {
      await result.current.save();
    });

    expect(spy.updateZoneCalls).toHaveLength(0);
  });

  it('tracks saving state', async () => {
    const zone = aWatchZone({ name: 'Home' });
    spy.updateZoneResult = aWatchZone({ name: 'Office' });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setName('Office');
    });

    expect(result.current.isSaving).toBe(false);

    await act(async () => {
      await result.current.save();
    });

    expect(result.current.isSaving).toBe(false);
  });

  it('sets error on save failure', async () => {
    const zone = aWatchZone({ name: 'Home' });
    spy.updateZoneError = new Error('Server error');

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setName('Office');
    });

    await act(async () => {
      await result.current.save();
    });

    expect(result.current.error).toBe('Server error');
  });

  it('resets dirty state after successful save', async () => {
    const zone = aWatchZone({ name: 'Home', radiusMetres: 2000 });
    spy.updateZoneResult = aWatchZone({ name: 'Office', radiusMetres: 2000 });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setName('Office');
    });

    expect(result.current.isDirty).toBe(true);

    await act(async () => {
      await result.current.save();
    });

    expect(result.current.isDirty).toBe(false);
  });

  it('validates that name cannot be empty', () => {
    const zone = aWatchZone({ name: 'Home' });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setName('   ');
    });

    expect(result.current.nameError).toBe('Zone name is required');
    expect(result.current.canSave).toBe(false);
  });

  it('canSave is true when dirty and valid', () => {
    const zone = aWatchZone({ name: 'Home' });

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    act(() => {
      result.current.setName('Office');
    });

    expect(result.current.canSave).toBe(true);
  });

  it('canSave is false when not dirty', () => {
    const zone = aWatchZone();

    const { result } = renderHook(() => useZoneEdit(spy, zone));

    expect(result.current.canSave).toBe(false);
  });

  describe('per-zone notification toggles', () => {
    it('initialises pushEnabled and emailInstantEnabled from the zone', () => {
      const zone = aWatchZone({ pushEnabled: false, emailInstantEnabled: true });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      expect(result.current.pushEnabled).toBe(false);
      expect(result.current.emailInstantEnabled).toBe(true);
      expect(result.current.isDirty).toBe(false);
    });

    it('toggling pushEnabled marks the form dirty', () => {
      const zone = aWatchZone({ pushEnabled: true, emailInstantEnabled: true });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      act(() => {
        result.current.setPushEnabled(false);
      });

      expect(result.current.pushEnabled).toBe(false);
      expect(result.current.isDirty).toBe(true);
    });

    it('toggling emailInstantEnabled marks the form dirty', () => {
      const zone = aWatchZone({ pushEnabled: true, emailInstantEnabled: true });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      act(() => {
        result.current.setEmailInstantEnabled(false);
      });

      expect(result.current.emailInstantEnabled).toBe(false);
      expect(result.current.isDirty).toBe(true);
    });

    it('sends pushEnabled in the patch payload when changed', async () => {
      const zone = aWatchZone({ pushEnabled: true, emailInstantEnabled: true });
      spy.updateZoneResult = aWatchZone({ pushEnabled: false, emailInstantEnabled: true });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      act(() => {
        result.current.setPushEnabled(false);
      });

      await act(async () => {
        await result.current.save();
      });

      expect(spy.updateZoneCalls[0]?.data).toEqual({ pushEnabled: false });
    });

    it('sends emailInstantEnabled in the patch payload when changed', async () => {
      const zone = aWatchZone({ pushEnabled: true, emailInstantEnabled: true });
      spy.updateZoneResult = aWatchZone({ pushEnabled: true, emailInstantEnabled: false });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      act(() => {
        result.current.setEmailInstantEnabled(false);
      });

      await act(async () => {
        await result.current.save();
      });

      expect(spy.updateZoneCalls[0]?.data).toEqual({ emailInstantEnabled: false });
    });

    it('omits unchanged notification flags from the patch payload', async () => {
      const zone = aWatchZone({
        name: 'Home',
        pushEnabled: true,
        emailInstantEnabled: true,
      });
      spy.updateZoneResult = aWatchZone({
        name: 'Office',
        pushEnabled: true,
        emailInstantEnabled: true,
      });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      act(() => {
        result.current.setName('Office');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(spy.updateZoneCalls[0]?.data).toEqual({ name: 'Office' });
    });

    it('resets dirty state after save and reflects updated flags', async () => {
      const zone = aWatchZone({ pushEnabled: true, emailInstantEnabled: true });
      spy.updateZoneResult = aWatchZone({ pushEnabled: false, emailInstantEnabled: true });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      act(() => {
        result.current.setPushEnabled(false);
      });

      expect(result.current.isDirty).toBe(true);

      await act(async () => {
        await result.current.save();
      });

      expect(result.current.isDirty).toBe(false);
      expect(result.current.pushEnabled).toBe(false);
    });
  });

  describe('shape mode / boundary', () => {
    it('initialises shapeMode as circle for a plain circle zone', () => {
      const zone = aWatchZone({ boundary: null });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      expect(result.current.shapeMode).toBe('circle');
      expect(result.current.isDirty).toBe(false);
    });

    it('initialises shapeMode as custom for a zone with an existing boundary', () => {
      const boundary = aWatchZoneBoundary();
      const zone = aWatchZone({ boundary });

      const { result } = renderHook(() => useZoneEdit(spy, zone, boundary));

      expect(result.current.shapeMode).toBe('custom');
      expect(result.current.isDirty).toBe(false);
    });

    it('marks the form dirty when switching shape mode', () => {
      const zone = aWatchZone({ boundary: null });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      act(() => {
        result.current.setShapeMode('custom');
      });

      expect(result.current.isDirty).toBe(true);
    });

    it('blocks save with a boundaryError when custom mode has no boundary yet', () => {
      const zone = aWatchZone({ boundary: null });

      const { result } = renderHook(() => useZoneEdit(spy, zone));

      act(() => {
        result.current.setShapeMode('custom');
      });

      expect(result.current.boundaryError).toBe(
        'Draw a shape with at least 3 points, then close it by clicking the first point again.',
      );
      expect(result.current.canSave).toBe(false);
    });

    it('sends an explicit null boundary when switching a custom zone back to circle', async () => {
      const boundary = aWatchZoneBoundary();
      const zone = aWatchZone({ boundary });
      spy.updateZoneResult = aWatchZone({ boundary: null });

      const { result } = renderHook(() => useZoneEdit(spy, zone, boundary));

      act(() => {
        result.current.setShapeMode('circle');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(spy.updateZoneCalls).toHaveLength(1);
      expect(spy.updateZoneCalls[0]?.data).toEqual({ boundary: null });
    });

    it('sends the new boundary when switching a circle zone to custom', async () => {
      const boundary = aWatchZoneBoundary();
      const zone = aWatchZone({ boundary: null });
      spy.updateZoneResult = aWatchZone({ boundary });

      const { result, rerender } = renderHook(
        ({ currentBoundary }) => useZoneEdit(spy, zone, currentBoundary),
        { initialProps: { currentBoundary: null as typeof boundary | null } },
      );

      act(() => {
        result.current.setShapeMode('custom');
      });
      rerender({ currentBoundary: boundary });

      await act(async () => {
        await result.current.save();
      });

      expect(spy.updateZoneCalls).toHaveLength(1);
      expect(spy.updateZoneCalls[0]?.data).toEqual({ boundary });
    });

    it('omits boundary from the patch when shape mode and boundary are both unchanged', async () => {
      const boundary = aWatchZoneBoundary();
      const zone = aWatchZone({ name: 'Home', boundary });
      spy.updateZoneResult = aWatchZone({ name: 'Office', boundary });

      const { result } = renderHook(() => useZoneEdit(spy, zone, boundary));

      act(() => {
        result.current.setName('Office');
      });

      await act(async () => {
        await result.current.save();
      });

      expect(spy.updateZoneCalls[0]?.data).toEqual({ name: 'Office' });
    });

    it('sends an updated boundary when a custom zone is redrawn', async () => {
      const originalBoundary = aWatchZoneBoundary();
      const redrawnBoundary = aWatchZoneBoundary({
        coordinates: [
          [
            [0.2, 52.3],
            [0.22, 52.3],
            [0.22, 52.31],
            [0.2, 52.3],
          ],
        ],
      });
      const zone = aWatchZone({ boundary: originalBoundary });
      spy.updateZoneResult = aWatchZone({ boundary: redrawnBoundary });

      const { result, rerender } = renderHook(
        ({ currentBoundary }) => useZoneEdit(spy, zone, currentBoundary),
        { initialProps: { currentBoundary: originalBoundary } },
      );

      expect(result.current.isDirty).toBe(false);

      rerender({ currentBoundary: redrawnBoundary });

      expect(result.current.isDirty).toBe(true);

      await act(async () => {
        await result.current.save();
      });

      expect(spy.updateZoneCalls[0]?.data).toEqual({ boundary: redrawnBoundary });
    });

    it('resets baseline shape state after a successful save', async () => {
      const boundary = aWatchZoneBoundary();
      const zone = aWatchZone({ boundary });
      spy.updateZoneResult = aWatchZone({ boundary: null });

      const { result } = renderHook(() => useZoneEdit(spy, zone, boundary));

      act(() => {
        result.current.setShapeMode('circle');
      });

      expect(result.current.isDirty).toBe(true);

      await act(async () => {
        await result.current.save();
      });

      expect(result.current.isDirty).toBe(false);
      expect(result.current.shapeMode).toBe('circle');
    });
  });
});
