import { renderHook, act } from '@testing-library/react';
import { useOnboarding } from '../useOnboarding';
import { SpyOnboardingPort } from './spies/spy-onboarding-port';
import type { GeocodeResult } from '../../../domain/types';

const geocodeResult: GeocodeResult = { latitude: 51.5074, longitude: -0.1278 };

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
});
