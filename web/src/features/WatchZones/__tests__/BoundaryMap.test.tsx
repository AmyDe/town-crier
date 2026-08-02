import type { ReactNode } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { BoundaryMap } from '../BoundaryMap';

interface LatLngPayload {
  latlng: { lat: number; lng: number };
}

interface DragEndPayload {
  target: { getLatLng: () => { lat: number; lng: number } };
}

interface MockMarkerEventHandlers {
  click?: () => void;
  dragend?: (event: DragEndPayload) => void;
}

interface MockMarkerProps {
  position: [number, number];
  eventHandlers?: MockMarkerEventHandlers;
}

interface PathLayerProps {
  positions: [number, number][];
}

const { capturedMapEvents } = vi.hoisted(() => ({
  capturedMapEvents: { click: undefined as ((event: LatLngPayload) => void) | undefined },
}));

vi.mock('react-leaflet', () => ({
  MapContainer: ({ children }: { children: ReactNode }) => (
    <div data-testid="map-container">{children}</div>
  ),
  TileLayer: () => <div data-testid="tile-layer" />,
  Marker: ({ position, eventHandlers }: MockMarkerProps) => (
    <div data-testid="vertex-marker" data-lat={position[0]} data-lng={position[1]}>
      <button type="button" data-testid="marker-click" onClick={() => eventHandlers?.click?.()}>
        marker
      </button>
      <button
        type="button"
        data-testid="marker-dragend"
        onClick={() =>
          eventHandlers?.dragend?.({ target: { getLatLng: () => ({ lat: 52.3, lng: 0.2 }) } })
        }
      >
        drag
      </button>
    </div>
  ),
  Polygon: ({ positions }: PathLayerProps) => (
    <div data-testid="polygon" data-count={positions.length} />
  ),
  Polyline: ({ positions }: PathLayerProps) => (
    <div data-testid="polyline" data-count={positions.length} />
  ),
  useMapEvents: (handlers: { click?: (event: LatLngPayload) => void }) => {
    capturedMapEvents.click = handlers.click;
    return null;
  },
}));

describe('BoundaryMap', () => {
  const centre = { latitude: 52.2053, longitude: 0.1218 };

  it('renders the map container and tile layer', () => {
    render(
      <BoundaryMap
        centre={centre}
        vertices={[]}
        isClosed={false}
        onAddVertex={() => {}}
        onMoveVertex={() => {}}
        onCloseRing={() => {}}
      />,
    );

    expect(screen.getByTestId('map-container')).toBeInTheDocument();
    expect(screen.getByTestId('tile-layer')).toBeInTheDocument();
  });

  it('renders one marker per vertex', () => {
    render(
      <BoundaryMap
        centre={centre}
        vertices={[
          { latitude: 52.2, longitude: 0.1 },
          { latitude: 52.21, longitude: 0.12 },
        ]}
        isClosed={false}
        onAddVertex={() => {}}
        onMoveVertex={() => {}}
        onCloseRing={() => {}}
      />,
    );

    expect(screen.getAllByTestId('vertex-marker')).toHaveLength(2);
  });

  it('renders an open dashed line while the ring has 2+ vertices and is not closed', () => {
    render(
      <BoundaryMap
        centre={centre}
        vertices={[
          { latitude: 52.2, longitude: 0.1 },
          { latitude: 52.21, longitude: 0.12 },
        ]}
        isClosed={false}
        onAddVertex={() => {}}
        onMoveVertex={() => {}}
        onCloseRing={() => {}}
      />,
    );

    expect(screen.getByTestId('polyline')).toBeInTheDocument();
    expect(screen.queryByTestId('polygon')).not.toBeInTheDocument();
  });

  it('renders a filled polygon once the ring is closed', () => {
    render(
      <BoundaryMap
        centre={centre}
        vertices={[
          { latitude: 52.2, longitude: 0.1 },
          { latitude: 52.2, longitude: 0.12 },
          { latitude: 52.21, longitude: 0.12 },
        ]}
        isClosed={true}
        onAddVertex={() => {}}
        onMoveVertex={() => {}}
        onCloseRing={() => {}}
      />,
    );

    expect(screen.getByTestId('polygon')).toBeInTheDocument();
    expect(screen.queryByTestId('polyline')).not.toBeInTheDocument();
  });

  it('calls onAddVertex when the map is clicked', () => {
    const onAddVertex = vi.fn();

    render(
      <BoundaryMap
        centre={centre}
        vertices={[]}
        isClosed={false}
        onAddVertex={onAddVertex}
        onMoveVertex={() => {}}
        onCloseRing={() => {}}
      />,
    );

    capturedMapEvents.click?.({ latlng: { lat: 52.25, lng: 0.15 } });

    expect(onAddVertex).toHaveBeenCalledWith({ latitude: 52.25, longitude: 0.15 });
  });

  it('calls onCloseRing when the first vertex marker is clicked', async () => {
    const user = userEvent.setup();
    const onCloseRing = vi.fn();

    render(
      <BoundaryMap
        centre={centre}
        vertices={[
          { latitude: 52.2, longitude: 0.1 },
          { latitude: 52.21, longitude: 0.12 },
        ]}
        isClosed={false}
        onAddVertex={() => {}}
        onMoveVertex={() => {}}
        onCloseRing={onCloseRing}
      />,
    );

    const clickButtons = screen.getAllByTestId('marker-click');
    await user.click(clickButtons[0]!);

    expect(onCloseRing).toHaveBeenCalledTimes(1);
  });

  it('does not wire a close handler onto markers after the first', async () => {
    const user = userEvent.setup();
    const onCloseRing = vi.fn();

    render(
      <BoundaryMap
        centre={centre}
        vertices={[
          { latitude: 52.2, longitude: 0.1 },
          { latitude: 52.21, longitude: 0.12 },
        ]}
        isClosed={false}
        onAddVertex={() => {}}
        onMoveVertex={() => {}}
        onCloseRing={onCloseRing}
      />,
    );

    const clickButtons = screen.getAllByTestId('marker-click');
    await user.click(clickButtons[1]!);

    expect(onCloseRing).not.toHaveBeenCalled();
  });

  it('calls onMoveVertex with the vertex index and new position when a marker is dragged', async () => {
    const user = userEvent.setup();
    const onMoveVertex = vi.fn();

    render(
      <BoundaryMap
        centre={centre}
        vertices={[
          { latitude: 52.2, longitude: 0.1 },
          { latitude: 52.21, longitude: 0.12 },
        ]}
        isClosed={false}
        onAddVertex={() => {}}
        onMoveVertex={onMoveVertex}
        onCloseRing={() => {}}
      />,
    );

    const dragButtons = screen.getAllByTestId('marker-dragend');
    await user.click(dragButtons[1]!);

    expect(onMoveVertex).toHaveBeenCalledWith(1, { latitude: 52.3, longitude: 0.2 });
  });
});
