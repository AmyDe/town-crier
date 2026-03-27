import { renderHook, act, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { usePostcodeGeocode } from '../usePostcodeGeocode';
import { SpyGeocodingPort } from './spies/spy-geocoding-port';

describe('usePostcodeGeocode', () => {
  it('starts with empty postcode and no loading/error state', () => {
    const spy = new SpyGeocodingPort();

    const { result } = renderHook(() => usePostcodeGeocode(spy));

    expect(result.current.postcode).toBe('');
    expect(result.current.isGeocoding).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('geocodes the postcode and returns coordinates', async () => {
    const spy = new SpyGeocodingPort();
    spy.geocodeResult = { latitude: 51.5074, longitude: -0.1278 };

    const { result } = renderHook(() => usePostcodeGeocode(spy));

    act(() => {
      result.current.setPostcode('SW1A 1AA');
    });

    let geocodeResult: { latitude: number; longitude: number } | null = null;
    await act(async () => {
      geocodeResult = await result.current.lookup();
    });

    expect(geocodeResult).toEqual({ latitude: 51.5074, longitude: -0.1278 });
    expect(spy.geocodeCalls).toEqual(['SW1A 1AA']);
  });

  it('sets error on geocode failure', async () => {
    const spy = new SpyGeocodingPort();
    spy.geocodeError = new Error('Postcode not found');

    const { result } = renderHook(() => usePostcodeGeocode(spy));

    act(() => {
      result.current.setPostcode('INVALID');
    });

    await act(async () => {
      await result.current.lookup();
    });

    await waitFor(() => {
      expect(result.current.error).toBe('Postcode not found');
    });
  });

  it('clears error when postcode changes', async () => {
    const spy = new SpyGeocodingPort();
    spy.geocodeError = new Error('Postcode not found');

    const { result } = renderHook(() => usePostcodeGeocode(spy));

    act(() => {
      result.current.setPostcode('INVALID');
    });

    await act(async () => {
      await result.current.lookup();
    });

    await waitFor(() => {
      expect(result.current.error).toBe('Postcode not found');
    });

    act(() => {
      result.current.setPostcode('SW1A 1AA');
    });

    expect(result.current.error).toBeNull();
  });

  it('sets isGeocoding while lookup is in progress', async () => {
    const spy = new SpyGeocodingPort();
    let resolveGeocode: (value: { latitude: number; longitude: number }) => void;
    spy.geocode = (postcode: string) => {
      spy.geocodeCalls.push(postcode);
      return new Promise((resolve) => {
        resolveGeocode = resolve;
      });
    };

    const { result } = renderHook(() => usePostcodeGeocode(spy));

    act(() => {
      result.current.setPostcode('CB1 2AD');
    });

    let lookupPromise: Promise<unknown>;
    act(() => {
      lookupPromise = result.current.lookup();
    });

    expect(result.current.isGeocoding).toBe(true);

    await act(async () => {
      resolveGeocode!({ latitude: 52.2, longitude: 0.12 });
      await lookupPromise!;
    });

    expect(result.current.isGeocoding).toBe(false);
  });
});
