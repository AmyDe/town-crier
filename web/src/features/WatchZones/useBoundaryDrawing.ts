import { useState, useCallback, useMemo } from 'react';
import type { Coordinates, WatchZoneBoundary } from '../../domain/types';

/**
 * 3 is the geometric minimum for a polygon — matches the server-side
 * validation range (3 to 50 vertices) in GH#1031.
 */
const MIN_VERTICES = 3;

export interface BoundaryDrawingState {
  /** The in-progress or completed ring, in draw order. Never includes the
   * GeoJSON closing duplicate of the first vertex — that is added only when
   * computing `boundary`. */
  readonly vertices: readonly Coordinates[];
  /** True once the ring has been closed (first vertex clicked again). */
  readonly isClosed: boolean;
  /** True once there are enough vertices to legally close the ring. */
  readonly canClose: boolean;
  /**
   * The GeoJSON `Polygon` for the completed ring, or `null` while the ring
   * is still open or has too few vertices.
   */
  readonly boundary: WatchZoneBoundary | null;
  /** Click to add a vertex. Ignored once the ring is closed. */
  addVertex: (coordinate: Coordinates) => void;
  /** Drag a vertex marker to a new position. */
  moveVertex: (index: number, coordinate: Coordinates) => void;
  /** Click the first vertex again to close the ring. No-op below the minimum. */
  closeRing: () => void;
  /** Undo the last action: reopens a closed ring, or removes the last vertex. */
  undo: () => void;
  /** Clear all vertices and reopen the ring. */
  reset: () => void;
}

interface DrawingState {
  vertices: Coordinates[];
  isClosed: boolean;
}

function initialDrawingState(initialBoundary?: WatchZoneBoundary | null): DrawingState {
  if (!initialBoundary) {
    return { vertices: [], isClosed: false };
  }
  const ring = initialBoundary.coordinates[0] ?? [];
  // GeoJSON repeats the first vertex at the end to close the ring; drop it
  // since `vertices` always holds the open, de-duplicated point list.
  const openRing = ring.length > 1 ? ring.slice(0, -1) : ring;
  return {
    vertices: openRing.map(([longitude, latitude]) => ({ latitude, longitude })),
    isClosed: true,
  };
}

export function useBoundaryDrawing(
  initialBoundary?: WatchZoneBoundary | null,
): BoundaryDrawingState {
  const [state, setState] = useState<DrawingState>(() => initialDrawingState(initialBoundary));

  const addVertex = useCallback((coordinate: Coordinates) => {
    setState((prev) => {
      if (prev.isClosed) {
        return prev;
      }
      return { ...prev, vertices: [...prev.vertices, coordinate] };
    });
  }, []);

  const moveVertex = useCallback((index: number, coordinate: Coordinates) => {
    setState((prev) => ({
      ...prev,
      vertices: prev.vertices.map((vertex, i) => (i === index ? coordinate : vertex)),
    }));
  }, []);

  const closeRing = useCallback(() => {
    setState((prev) => {
      if (prev.isClosed || prev.vertices.length < MIN_VERTICES) {
        return prev;
      }
      return { ...prev, isClosed: true };
    });
  }, []);

  const undo = useCallback(() => {
    setState((prev) => {
      if (prev.isClosed) {
        return { ...prev, isClosed: false };
      }
      return { ...prev, vertices: prev.vertices.slice(0, -1) };
    });
  }, []);

  const reset = useCallback(() => {
    setState({ vertices: [], isClosed: false });
  }, []);

  const { vertices, isClosed } = state;
  const canClose = vertices.length >= MIN_VERTICES;

  const boundary = useMemo<WatchZoneBoundary | null>(() => {
    const firstVertex = vertices[0];
    if (!isClosed || !firstVertex || vertices.length < MIN_VERTICES) {
      return null;
    }
    const ring: (readonly [number, number])[] = vertices.map(
      (vertex) => [vertex.longitude, vertex.latitude] as const,
    );
    ring.push([firstVertex.longitude, firstVertex.latitude]);
    return { type: 'Polygon', coordinates: [ring] };
  }, [vertices, isClosed]);

  return {
    vertices,
    isClosed,
    canClose,
    boundary,
    addVertex,
    moveVertex,
    closeRing,
    undo,
    reset,
  };
}
