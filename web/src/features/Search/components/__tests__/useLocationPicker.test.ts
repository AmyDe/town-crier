import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect, vi, afterEach } from 'vitest';
import { useLocationPicker } from '../useLocationPicker';
import { SpyPostcodeLookupPort } from '../../__tests__/spies/spy-postcode-lookup-port';

interface StubGeolocation {
  getCurrentPosition: (
    success: (position: GeolocationPosition) => void,
    error?: (error: GeolocationPositionError) => void,
  ) => void;
}

function stubGeolocationSuccess(lat: number, lon: number): StubGeolocation {
  return {
    getCurrentPosition: (success) => {
      success({ coords: { latitude: lat, longitude: lon } } as GeolocationPosition);
    },
  };
}

function stubGeolocationDenied(): StubGeolocation {
  return {
    getCurrentPosition: (_success, error) => {
      error?.({ code: 1, message: 'User denied Geolocation' } as GeolocationPositionError);
    },
  };
}

describe('useLocationPicker', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('does not look up while the postcode is too short', async () => {
    const port = new SpyPostcodeLookupPort();
    const onConfirm = vi.fn();
    const { result } = renderHook(() => useLocationPicker(port, onConfirm));

    act(() => {
      result.current.setPostcode('SW1');
    });

    await new Promise((r) => setTimeout(r, 600));
    expect(port.lookupCalls).toHaveLength(0);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('debounces postcode entry into a single lookup, then confirms with the postcode as the label', async () => {
    const port = new SpyPostcodeLookupPort();
    port.lookupResult = { lat: 51.501, lon: -0.1415 };
    const onConfirm = vi.fn();
    const { result } = renderHook(() => useLocationPicker(port, onConfirm));

    act(() => {
      result.current.setPostcode('S');
    });
    act(() => {
      result.current.setPostcode('SW');
    });
    act(() => {
      result.current.setPostcode('sw1a 1aa');
    });

    await waitFor(
      () => {
        expect(port.lookupCalls).toHaveLength(1);
      },
      { timeout: 2000 },
    );
    expect(port.lookupCalls[0]).toBe('sw1a 1aa');

    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledWith({ lat: 51.501, lon: -0.1415 }, 'SW1A 1AA');
    });
  });

  it('surfaces a distinct error and does not confirm when the postcode is not found', async () => {
    const port = new SpyPostcodeLookupPort();
    port.lookupError = new Error('Postcode not found');
    const onConfirm = vi.fn();
    const { result } = renderHook(() => useLocationPicker(port, onConfirm));

    act(() => {
      result.current.setPostcode('zz9 9zz');
    });

    await waitFor(() => {
      expect(result.current.postcodeError).toBe('Postcode not found');
    });
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('surfaces a network-failure error distinctly from not-found', async () => {
    const port = new SpyPostcodeLookupPort();
    port.lookupError = new Error('Could not look up that postcode. Check your connection and try again.');
    const onConfirm = vi.fn();
    const { result } = renderHook(() => useLocationPicker(port, onConfirm));

    act(() => {
      result.current.setPostcode('sw1a 1aa');
    });

    await waitFor(() => {
      expect(result.current.postcodeError).toBe(
        'Could not look up that postcode. Check your connection and try again.',
      );
    });
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('confirms with "your location" on a successful geolocation request', async () => {
    vi.stubGlobal('navigator', { geolocation: stubGeolocationSuccess(51.5, -0.12) });
    const port = new SpyPostcodeLookupPort();
    const onConfirm = vi.fn();
    const { result } = renderHook(() => useLocationPicker(port, onConfirm));

    act(() => {
      result.current.useMyLocation();
    });

    expect(onConfirm).toHaveBeenCalledWith({ lat: 51.5, lon: -0.12 }, 'your location');
  });

  it('surfaces an error and does not confirm when geolocation is denied', () => {
    vi.stubGlobal('navigator', { geolocation: stubGeolocationDenied() });
    const port = new SpyPostcodeLookupPort();
    const onConfirm = vi.fn();
    const { result } = renderHook(() => useLocationPicker(port, onConfirm));

    act(() => {
      result.current.useMyLocation();
    });

    expect(onConfirm).not.toHaveBeenCalled();
    expect(result.current.geolocationError).not.toBeNull();
  });

  it('surfaces an error when the browser has no geolocation support', () => {
    vi.stubGlobal('navigator', {});
    const port = new SpyPostcodeLookupPort();
    const onConfirm = vi.fn();
    const { result } = renderHook(() => useLocationPicker(port, onConfirm));

    act(() => {
      result.current.useMyLocation();
    });

    expect(onConfirm).not.toHaveBeenCalled();
    expect(result.current.geolocationError).not.toBeNull();
  });

  it('confirms a map tap with a "custom pin" label', () => {
    const port = new SpyPostcodeLookupPort();
    const onConfirm = vi.fn();
    const { result } = renderHook(() => useLocationPicker(port, onConfirm));

    act(() => {
      result.current.selectMapPoint(52.2053, 0.1218);
    });

    expect(onConfirm).toHaveBeenCalledWith({ lat: 52.2053, lon: 0.1218 }, 'custom pin');
  });
});
