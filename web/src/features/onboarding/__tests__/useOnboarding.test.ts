import { renderHook, act } from '@testing-library/react';
import { useOnboarding } from '../useOnboarding';
import { SpyOnboardingPort } from './spies/spy-onboarding-port';
import type { GeocodeResult, WatchZoneBoundary } from '../../../domain/types';

const geocodeResult: GeocodeResult = { latitude: 51.5074, longitude: -0.1278 };

const closedSquareBoundary: WatchZoneBoundary = {
  type: 'Polygon',
  coordinates: [
    [
      [0.1, 52.2],
      [0.12, 52.2],
      [0.12, 52.21],
      [0.1, 52.2],
    ],
  ],
};

describe('useOnboarding', () => {
  it('starts at the welcome step', () => {
    const spy = new SpyOnboardingPort();
    const { result } = renderHook(() => useOnboarding(spy));

    expect(result.current.step).toBe('welcome');
    expect(result.current.isComplete).toBe(false);
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('advances to postcode step when start is called', () => {
    const spy = new SpyOnboardingPort();
    const { result } = renderHook(() => useOnboarding(spy));

    act(() => {
      result.current.start();
    });

    expect(result.current.step).toBe('postcode');
  });

  it('advances to radius step when geocode result is provided', () => {
    const spy = new SpyOnboardingPort();
    const { result } = renderHook(() => useOnboarding(spy));

    act(() => {
      result.current.start();
    });
    act(() => {
      result.current.handleGeocode(geocodeResult);
    });

    expect(result.current.step).toBe('radius');
    expect(result.current.geocode).toEqual(geocodeResult);
  });

  it('tracks radius selection and advances to confirm', () => {
    const spy = new SpyOnboardingPort();
    const { result } = renderHook(() => useOnboarding(spy));

    act(() => {
      result.current.start();
    });
    act(() => {
      result.current.handleGeocode(geocodeResult);
    });
    act(() => {
      result.current.selectRadius(5000);
    });

    expect(result.current.radiusMetres).toBe(5000);

    act(() => {
      result.current.confirmRadius();
    });

    expect(result.current.step).toBe('confirm');
  });

  it('calls createProfile and createWatchZone on finish', async () => {
    const spy = new SpyOnboardingPort();
    const { result } = renderHook(() => useOnboarding(spy));

    // Advance to confirm
    act(() => { result.current.start(); });
    act(() => { result.current.handleGeocode(geocodeResult); });
    act(() => { result.current.selectRadius(2000); });
    act(() => { result.current.confirmRadius(); });

    await act(async () => {
      await result.current.finish();
    });

    expect(spy.createProfileCalls).toBe(1);
    expect(spy.createWatchZoneCalls).toHaveLength(1);
    expect(spy.createWatchZoneCalls[0]).toEqual({
      name: 'Home',
      latitude: 51.5074,
      longitude: -0.1278,
      radiusMetres: 2000,
    });
    expect(result.current.isComplete).toBe(true);
    expect(result.current.isSubmitting).toBe(false);
  });

  it('sets error when finish fails', async () => {
    const spy = new SpyOnboardingPort();
    spy.createProfileError = new Error('Network error');
    const { result } = renderHook(() => useOnboarding(spy));

    act(() => { result.current.start(); });
    act(() => { result.current.handleGeocode(geocodeResult); });
    act(() => { result.current.selectRadius(1000); });
    act(() => { result.current.confirmRadius(); });

    await act(async () => {
      await result.current.finish();
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.isComplete).toBe(false);
    expect(result.current.isSubmitting).toBe(false);
  });

  it('defaults radius to 1000 metres', () => {
    const spy = new SpyOnboardingPort();
    const { result } = renderHook(() => useOnboarding(spy));

    expect(result.current.radiusMetres).toBe(1000);
  });

  it('defaults tier to Free', () => {
    const spy = new SpyOnboardingPort();
    const { result } = renderHook(() => useOnboarding(spy));

    expect(result.current.tier).toBe('Free');
  });

  describe('upgradeTier', () => {
    // There is no purchase/checkout UI on web yet (GH#1031 section 8) — no
    // real trigger calls this today. These tests exercise the plumbing
    // directly so a future purchase-completion callback has somewhere to
    // hook in without restarting onboarding.

    it('swaps the current step from radius to drawing mid-flow', () => {
      const spy = new SpyOnboardingPort();
      const { result } = renderHook(() => useOnboarding(spy));

      act(() => { result.current.start(); });
      act(() => { result.current.handleGeocode(geocodeResult, 'SW1A 1AA'); });
      expect(result.current.step).toBe('radius');

      act(() => {
        result.current.upgradeTier('Personal');
      });

      expect(result.current.step).toBe('drawing');
      expect(result.current.tier).toBe('Personal');
    });

    it('updates the tier without moving the step when not on the radius step', () => {
      const spy = new SpyOnboardingPort();
      const { result } = renderHook(() => useOnboarding(spy));

      expect(result.current.step).toBe('welcome');

      act(() => {
        result.current.upgradeTier('Pro');
      });

      expect(result.current.step).toBe('welcome');
      expect(result.current.tier).toBe('Pro');
    });

    it('does not move off radius when "upgrading" back to Free', () => {
      const spy = new SpyOnboardingPort();
      const { result } = renderHook(() => useOnboarding(spy));

      act(() => { result.current.start(); });
      act(() => { result.current.handleGeocode(geocodeResult, 'SW1A 1AA'); });
      expect(result.current.step).toBe('radius');

      act(() => {
        result.current.upgradeTier('Free');
      });

      expect(result.current.step).toBe('radius');
      expect(result.current.tier).toBe('Free');
    });
  });

  describe('confirmDrawing', () => {
    it('advances to confirm when a boundary is present', () => {
      const spy = new SpyOnboardingPort();
      const { result } = renderHook(() => useOnboarding(spy));

      act(() => { result.current.start(); });
      act(() => { result.current.handleGeocode(geocodeResult, 'SW1A 1AA'); });
      act(() => { result.current.upgradeTier('Personal'); });
      expect(result.current.step).toBe('drawing');

      act(() => {
        result.current.confirmDrawing(closedSquareBoundary);
      });

      expect(result.current.step).toBe('confirm');
      expect(result.current.error).toBeNull();
    });

    it('sets an error and stays on the drawing step when no boundary is present', () => {
      const spy = new SpyOnboardingPort();
      const { result } = renderHook(() => useOnboarding(spy));

      act(() => { result.current.start(); });
      act(() => { result.current.handleGeocode(geocodeResult, 'SW1A 1AA'); });
      act(() => { result.current.upgradeTier('Personal'); });

      act(() => {
        result.current.confirmDrawing(null);
      });

      expect(result.current.step).toBe('drawing');
      expect(result.current.error).not.toBeNull();
    });
  });

  describe('finish with a boundary', () => {
    it('includes the boundary in the createWatchZone call when one was drawn', async () => {
      const spy = new SpyOnboardingPort();
      const { result } = renderHook(() => useOnboarding(spy));

      act(() => { result.current.start(); });
      act(() => { result.current.handleGeocode(geocodeResult, 'SW1A 1AA'); });
      act(() => { result.current.upgradeTier('Personal'); });
      act(() => { result.current.confirmDrawing(closedSquareBoundary); });

      await act(async () => {
        await result.current.finish(closedSquareBoundary);
      });

      expect(spy.createWatchZoneCalls).toHaveLength(1);
      expect(spy.createWatchZoneCalls[0]).toEqual({
        name: 'Home',
        latitude: 51.5074,
        longitude: -0.1278,
        radiusMetres: 1000,
        boundary: closedSquareBoundary,
      });
      expect(result.current.isComplete).toBe(true);
    });

    it('omits the boundary key entirely when finishing without one (unchanged circle behaviour)', async () => {
      const spy = new SpyOnboardingPort();
      const { result } = renderHook(() => useOnboarding(spy));

      act(() => { result.current.start(); });
      act(() => { result.current.handleGeocode(geocodeResult, 'SW1A 1AA'); });
      act(() => { result.current.selectRadius(2000); });
      act(() => { result.current.confirmRadius(); });

      await act(async () => {
        await result.current.finish();
      });

      expect(spy.createWatchZoneCalls[0]).toEqual({
        name: 'Home',
        latitude: 51.5074,
        longitude: -0.1278,
        radiusMetres: 2000,
      });
    });
  });
});
