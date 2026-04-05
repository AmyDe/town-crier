import { useState, useCallback, useMemo } from 'react';
import type { ApplicationUid, PlanningApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { useFetchData } from '../../hooks/useFetchData';

interface MapData {
  readonly applications: readonly PlanningApplication[];
  readonly fetchedSavedUids: ReadonlySet<ApplicationUid>;
}

export function useMapData(port: MapPort) {
  const { data, isLoading, error, refresh } = useFetchData<MapData>(
    async () => {
      const [authorities, savedApps] = await Promise.all([
        port.fetchMyAuthorities(),
        port.fetchSavedApplications(),
      ]);

      const uniqueAuthorityIds = [...new Set(authorities.map(a => a.id))];
      const applicationArrays = await Promise.all(
        uniqueAuthorityIds.map(id => port.fetchApplicationsByAuthority(id)),
      );

      return {
        applications: applicationArrays.flat(),
        fetchedSavedUids: new Set(savedApps.map(s => s.applicationUid)),
      };
    },
    [port],
  );

  const [pendingSaves, setPendingSaves] = useState(new Set<ApplicationUid>());
  const [pendingRemoves, setPendingRemoves] = useState(new Set<ApplicationUid>());

  const savedUids: ReadonlySet<ApplicationUid> = useMemo(() => {
    const result = new Set(data?.fetchedSavedUids ?? []);
    for (const uid of pendingSaves) result.add(uid);
    for (const uid of pendingRemoves) result.delete(uid);
    return result;
  }, [data?.fetchedSavedUids, pendingSaves, pendingRemoves]);

  const saveApplication = useCallback(async (uid: ApplicationUid) => {
    setPendingSaves(prev => new Set([...prev, uid]));
    try {
      await port.saveApplication(uid);
    } catch {
      setPendingSaves(prev => {
        const next = new Set(prev);
        next.delete(uid);
        return next;
      });
    }
  }, [port]);

  const unsaveApplication = useCallback(async (uid: ApplicationUid) => {
    setPendingRemoves(prev => new Set([...prev, uid]));
    try {
      await port.unsaveApplication(uid);
    } catch {
      setPendingRemoves(prev => {
        const next = new Set(prev);
        next.delete(uid);
        return next;
      });
    }
  }, [port]);

  return {
    applications: data?.applications ?? [],
    savedUids,
    isLoading,
    error,
    refresh,
    saveApplication,
    unsaveApplication,
  };
}
