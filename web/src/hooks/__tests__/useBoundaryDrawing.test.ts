import { renderHook, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useBoundaryDrawing } from '../useBoundaryDrawing';
import type { WatchZoneBoundary } from '../../domain/types';

// Local closed-square boundary literal — deliberately not shared with
// WatchZones' `fixtures/watch-zone.fixtures.ts` (this hook lives under
// `hooks/`, which must not depend on a feature-private fixture).
function aWatchZoneBoundary(overrides?: Partial<WatchZoneBoundary>): WatchZoneBoundary {
  return {
    type: 'Polygon',
    coordinates: [
      [
        [0.1, 52.2],
        [0.12, 52.2],
        [0.12, 52.21],
        [0.1, 52.21],
        [0.1, 52.2],
      ],
    ],
    ...overrides,
  };
}

describe('useBoundaryDrawing', () => {
  it('starts with no vertices and an open ring', () => {
    const { result } = renderHook(() => useBoundaryDrawing());

    expect(result.current.vertices).toEqual([]);
    expect(result.current.isClosed).toBe(false);
    expect(result.current.canClose).toBe(false);
    expect(result.current.boundary).toBeNull();
  });

  it('adds a vertex on addVertex', () => {
    const { result } = renderHook(() => useBoundaryDrawing());

    act(() => {
      result.current.addVertex({ latitude: 52.2, longitude: 0.1 });
    });

    expect(result.current.vertices).toEqual([{ latitude: 52.2, longitude: 0.1 }]);
  });

  it('does not close the ring with fewer than 3 vertices (minimum-vertex guard)', () => {
    const { result } = renderHook(() => useBoundaryDrawing());

    act(() => {
      result.current.addVertex({ latitude: 52.2, longitude: 0.1 });
      result.current.addVertex({ latitude: 52.21, longitude: 0.12 });
    });

    expect(result.current.canClose).toBe(false);

    act(() => {
      result.current.closeRing();
    });

    expect(result.current.isClosed).toBe(false);
    expect(result.current.boundary).toBeNull();
  });

  it('closes the ring after 3 vertices via closeRing (clicking the first vertex)', () => {
    const { result } = renderHook(() => useBoundaryDrawing());

    act(() => {
      result.current.addVertex({ latitude: 52.2, longitude: 0.1 });
      result.current.addVertex({ latitude: 52.2, longitude: 0.12 });
      result.current.addVertex({ latitude: 52.21, longitude: 0.12 });
    });

    expect(result.current.canClose).toBe(true);

    act(() => {
      result.current.closeRing();
    });

    expect(result.current.isClosed).toBe(true);
    expect(result.current.boundary).toEqual({
      type: 'Polygon',
      coordinates: [
        [
          [0.1, 52.2],
          [0.12, 52.2],
          [0.12, 52.21],
          [0.1, 52.2],
        ],
      ],
    });
  });

  it('ignores addVertex once the ring is closed', () => {
    const { result } = renderHook(() => useBoundaryDrawing());

    act(() => {
      result.current.addVertex({ latitude: 52.2, longitude: 0.1 });
      result.current.addVertex({ latitude: 52.2, longitude: 0.12 });
      result.current.addVertex({ latitude: 52.21, longitude: 0.12 });
      result.current.closeRing();
    });

    act(() => {
      result.current.addVertex({ latitude: 53, longitude: 1 });
    });

    expect(result.current.vertices).toHaveLength(3);
  });

  it('moves a vertex via moveVertex (drag to adjust)', () => {
    const { result } = renderHook(() => useBoundaryDrawing());

    act(() => {
      result.current.addVertex({ latitude: 52.2, longitude: 0.1 });
      result.current.addVertex({ latitude: 52.21, longitude: 0.12 });
    });

    act(() => {
      result.current.moveVertex(0, { latitude: 52.25, longitude: 0.15 });
    });

    expect(result.current.vertices).toEqual([
      { latitude: 52.25, longitude: 0.15 },
      { latitude: 52.21, longitude: 0.12 },
    ]);
  });

  it('undo removes the last vertex when the ring is open', () => {
    const { result } = renderHook(() => useBoundaryDrawing());

    act(() => {
      result.current.addVertex({ latitude: 52.2, longitude: 0.1 });
      result.current.addVertex({ latitude: 52.21, longitude: 0.12 });
    });

    act(() => {
      result.current.undo();
    });

    expect(result.current.vertices).toEqual([{ latitude: 52.2, longitude: 0.1 }]);
  });

  it('undo reopens the ring when closed, without removing a vertex', () => {
    const { result } = renderHook(() => useBoundaryDrawing());

    act(() => {
      result.current.addVertex({ latitude: 52.2, longitude: 0.1 });
      result.current.addVertex({ latitude: 52.2, longitude: 0.12 });
      result.current.addVertex({ latitude: 52.21, longitude: 0.12 });
      result.current.closeRing();
    });

    act(() => {
      result.current.undo();
    });

    expect(result.current.isClosed).toBe(false);
    expect(result.current.vertices).toHaveLength(3);
    expect(result.current.boundary).toBeNull();
  });

  it('reset clears the vertices and reopens the ring', () => {
    const { result } = renderHook(() => useBoundaryDrawing());

    act(() => {
      result.current.addVertex({ latitude: 52.2, longitude: 0.1 });
      result.current.addVertex({ latitude: 52.2, longitude: 0.12 });
      result.current.addVertex({ latitude: 52.21, longitude: 0.12 });
      result.current.closeRing();
    });

    act(() => {
      result.current.reset();
    });

    expect(result.current.vertices).toEqual([]);
    expect(result.current.isClosed).toBe(false);
    expect(result.current.boundary).toBeNull();
  });

  it('initialises vertices and closed state from an existing boundary', () => {
    const { result } = renderHook(() => useBoundaryDrawing(aWatchZoneBoundary()));

    expect(result.current.isClosed).toBe(true);
    expect(result.current.vertices).toEqual([
      { latitude: 52.2, longitude: 0.1 },
      { latitude: 52.2, longitude: 0.12 },
      { latitude: 52.21, longitude: 0.12 },
      { latitude: 52.21, longitude: 0.1 },
    ]);
    expect(result.current.boundary).toEqual(aWatchZoneBoundary());
  });

  it('treats a null initial boundary the same as no boundary', () => {
    const { result } = renderHook(() => useBoundaryDrawing(null));

    expect(result.current.isClosed).toBe(false);
    expect(result.current.vertices).toEqual([]);
  });
});
