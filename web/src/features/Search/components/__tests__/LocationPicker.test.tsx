import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { LocationPicker } from '../LocationPicker';
import { SpyPostcodeLookupPort } from '../../__tests__/spies/spy-postcode-lookup-port';

const h = vi.hoisted(() => {
  let capturedClickHandler: ((event: { latlng: { lat: number; lng: number } }) => void) | null = null;
  return {
    setClickHandler: (fn: typeof capturedClickHandler) => {
      capturedClickHandler = fn;
    },
    fireMapClick: (lat: number, lng: number) => {
      capturedClickHandler?.({ latlng: { lat, lng } });
    },
  };
});

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ position }: { position: [number, number] }) => (
    <div data-testid="marker" data-lat={position[0]} data-lon={position[1]} />
  ),
  useMapEvents: (handlers: { click: (event: { latlng: { lat: number; lng: number } }) => void }) => {
    h.setClickHandler(handlers.click);
    return null;
  },
}));

describe('LocationPicker', () => {
  let port: SpyPostcodeLookupPort;
  let onConfirm: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    port = new SpyPostcodeLookupPort();
    onConfirm = vi.fn();
  });

  it('renders a postcode input, a "use my location" button, and a map', () => {
    render(<LocationPicker postcodePort={port} initialLocation={null} onConfirm={onConfirm} />);

    expect(screen.getByLabelText(/postcode/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /use my location/i })).toBeInTheDocument();
    expect(screen.getByTestId('map-container')).toBeInTheDocument();
  });

  it('confirms the resolved location after a debounced postcode lookup', async () => {
    port.lookupResult = { lat: 51.501, lon: -0.1415 };
    render(<LocationPicker postcodePort={port} initialLocation={null} onConfirm={onConfirm} />);

    fireEvent.change(screen.getByLabelText(/postcode/i), { target: { value: 'SW1A 1AA' } });

    await waitFor(
      () => {
        expect(onConfirm).toHaveBeenCalledWith({ lat: 51.501, lon: -0.1415 }, 'SW1A 1AA');
      },
      { timeout: 2000 },
    );
  });

  it('shows a distinct message when the postcode is not found', async () => {
    port.lookupError = new Error('Postcode not found');
    render(<LocationPicker postcodePort={port} initialLocation={null} onConfirm={onConfirm} />);

    fireEvent.change(screen.getByLabelText(/postcode/i), { target: { value: 'ZZ9 9ZZ' } });

    await waitFor(
      () => {
        expect(screen.getByText('Postcode not found')).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('confirms a "custom pin" when the map is tapped', () => {
    render(<LocationPicker postcodePort={port} initialLocation={null} onConfirm={onConfirm} />);

    h.fireMapClick(52.2053, 0.1218);

    expect(onConfirm).toHaveBeenCalledWith({ lat: 52.2053, lon: 0.1218 }, 'custom pin');
  });

  it('renders a marker prefilled at the current selection when reopened via Change', () => {
    render(
      <LocationPicker
        postcodePort={port}
        initialLocation={{ lat: 51.5, lon: -0.1 }}
        onConfirm={onConfirm}
      />,
    );

    const marker = screen.getByTestId('marker');
    expect(marker.getAttribute('data-lat')).toBe('51.5');
    expect(marker.getAttribute('data-lon')).toBe('-0.1');
  });

  it('never sends the postcode lookup through anything other than the injected port', async () => {
    render(<LocationPicker postcodePort={port} initialLocation={null} onConfirm={onConfirm} />);

    fireEvent.change(screen.getByLabelText(/postcode/i), { target: { value: 'SW1A 1AA' } });

    await waitFor(() => {
      expect(port.lookupCalls).toEqual(['SW1A 1AA']);
    });
  });
});
