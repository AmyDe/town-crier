import { useEffect, useRef } from 'react';
import { useMap } from 'react-leaflet';
import L from 'leaflet';

interface FitBoundsProps {
  positions: readonly [number, number][];
}

export function FitBounds({ positions }: FitBoundsProps) {
  const map = useMap();
  const hasFit = useRef(false);

  useEffect(() => {
    if (hasFit.current || positions.length === 0) return;
    const bounds = L.latLngBounds(
      positions.map(([lat, lng]) => L.latLng(lat, lng)),
    );
    map.fitBounds(bounds, { padding: [50, 50] });
    hasFit.current = true;
  }, [map, positions]);

  return null;
}
