import { useState, useMemo, type Dispatch, type SetStateAction } from 'react';
import type { ApplicationUid, PlanningApplication } from '../../domain/types';
import type { MapPort } from '../../domain/ports/map-port';
import { useFetchData } from '../../hooks/useFetchData';

interface MapData {
  readonly applications: readonly PlanningApplication[];
  readonly fetchedSavedUids: ReadonlySet<ApplicationUid>;
}

type UidSetSetter = Dispatch<SetStateAction<Set<ApplicationUid>>>;

function makeToggle(
  setAdd: UidSetSetter,
  setRemove: UidSetSetter,
  portAction: (uid: ApplicationUid) => Promise<void>,
) {
  return async (uid: ApplicationUid) => {
    setRemove(prev => {
      const next = new Set(prev);
      next.delete(uid);
      return next;
    });
    setAdd(prev => new Set([...prev, uid]));
    try {
      await portAction(uid);
    } catch {
      setAdd(prev => {
        const next = new Set(prev);
        next.delete(uid);
        return next;
      });
    }
  };
}

export function useMapData(port: MapPort) {
  const { data, isLoading, error, refresh } = useFetchData<MapData>(
    async () => {
      const [zones, savedApps] = await Promise.all([
        port.fetchMyZones(),
        port.fetchSavedApplications(),
      ]);

      const applicationArrays = await Promise.all(
        zones.map(z => port.fetchApplicationsByZone(z.id)),
      );

      // Deduplicate across zones (applications may appear in overlapping zones)
      const seen = new Set<string>();
      const deduped: PlanningApplication[] = [];
      for (const apps of applicationArrays) {
        for (const app of apps) {
          if (!seen.has(app.uid as string)) {
            seen.add(app.uid as string);
            deduped.push(app);
          }
        }
      }

      return {
        applications: deduped,
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

  const saveApplication = useMemo(
    () => makeToggle(setPendingSaves, setPendingRemoves, uid => port.saveApplication(uid)),
    [port],
  );

  const unsaveApplication = useMemo(
    () => makeToggle(setPendingRemoves, setPendingSaves, uid => port.unsaveApplication(uid)),
    [port],
  );

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
