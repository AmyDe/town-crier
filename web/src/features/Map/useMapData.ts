import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import type {
  ApplicationStatus,
  ClusterMember,
  MapCluster,
  PlanningApplication,
  WatchZoneId,
  WatchZoneSummary,
} from '../../domain/types';
import type { MapBounds, MapPort } from '../../domain/ports/map-port';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

/** Debounce window for viewport-driven cluster refetches (Leaflet move/zoom). */
const REGION_DEBOUNCE_MS = 250;

interface MapRegion {
  readonly bounds: MapBounds;
  readonly zoom: number;
}

/**
 * ViewModel for the watch-zone map: refetches server-side clusters for the
 * active zone's viewport (debounced) and point-reads a single-member pin.
 */
export function useMapData(port: MapPort) {
  const [zones, setZones] = useState<readonly WatchZoneSummary[]>([]);
  const [selectedZoneId, setSelectedZoneId] = useState<WatchZoneId | null>(null);
  const [clusters, setClusters] = useState<readonly MapCluster[]>([]);
  const [selectedStatusFilter, setSelectedStatusFilter] = useState<ApplicationStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Refs let the stable, debounced region handler read the latest zone/filter
  // without being recreated (which would detach Leaflet's event listeners).
  const selectedZoneIdRef = useRef<WatchZoneId | null>(null);
  selectedZoneIdRef.current = selectedZoneId;
  const statusFilterRef = useRef<ApplicationStatus | null>(null);
  statusFilterRef.current = selectedStatusFilter;
  const regionRef = useRef<MapRegion | null>(null);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const selectedZone = useMemo(
    () => zones.find((z) => z.id === selectedZoneId) ?? null,
    [zones, selectedZoneId],
  );

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setIsLoading(true);
      setError(null);
      try {
        const loaded = await port.fetchMyZones();
        if (cancelled) return;
        setZones(loaded);
        setSelectedZoneId(loaded[0]?.id ?? null);
      } catch (err: unknown) {
        if (!cancelled) setError(extractErrorMessage(err));
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [port]);

  const fetchClusters = useCallback(
    async (zoneId: WatchZoneId, region: MapRegion, status: ApplicationStatus | null) => {
      try {
        const result = await port.fetchClusters(zoneId, region.bounds, region.zoom, status);
        setClusters(result);
        setError(null);
      } catch (err: unknown) {
        // A transient pan/zoom refetch failure keeps the last good clusters
        // rather than blanking the map; surface an error only when there is
        // nothing to show yet.
        setClusters((prev) => {
          if (prev.length === 0) setError(extractErrorMessage(err));
          return prev;
        });
      }
    },
    [port],
  );

  const onRegionChange = useCallback(
    (bounds: MapBounds, zoom: number) => {
      regionRef.current = { bounds, zoom };
      if (debounceRef.current) clearTimeout(debounceRef.current);
      debounceRef.current = setTimeout(() => {
        const zoneId = selectedZoneIdRef.current;
        const region = regionRef.current;
        if (zoneId && region) {
          void fetchClusters(zoneId, region, statusFilterRef.current);
        }
      }, REGION_DEBOUNCE_MS);
    },
    [fetchClusters],
  );

  const setStatusFilter = useCallback(
    (status: ApplicationStatus | null) => {
      setSelectedStatusFilter(status);
      const zoneId = selectedZoneIdRef.current;
      const region = regionRef.current;
      if (zoneId && region) {
        void fetchClusters(zoneId, region, status);
      }
    },
    [fetchClusters],
  );

  const selectZone = useCallback(
    (zone: WatchZoneSummary) => {
      setSelectedZoneId(zone.id);
      setSelectedStatusFilter(null);
      selectedZoneIdRef.current = zone.id;
      statusFilterRef.current = null;
      const region = regionRef.current;
      if (region) {
        void fetchClusters(zone.id, region, null);
      }
    },
    [fetchClusters],
  );

  const resolveMember = useCallback(
    async (member: ClusterMember): Promise<PlanningApplication | null> => {
      try {
        return await port.fetchApplicationByMember(member);
      } catch {
        return null;
      }
    },
    [port],
  );

  useEffect(
    () => () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    },
    [],
  );

  return {
    zones,
    selectedZone,
    clusters,
    selectedStatusFilter,
    isLoading,
    error,
    onRegionChange,
    setStatusFilter,
    selectZone,
    resolveMember,
  };
}
