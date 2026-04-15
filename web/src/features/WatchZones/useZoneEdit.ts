import { useState, useCallback, useMemo } from 'react';
import type { WatchZoneSummary, UpdateWatchZoneRequest } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

export function useZoneEdit(repository: WatchZoneRepository, zone: WatchZoneSummary) {
  const [baselineName, setBaselineName] = useState(zone.name);
  const [baselineRadius, setBaselineRadius] = useState(zone.radiusMetres);
  const [name, setName] = useState(zone.name);
  const [radiusMetres, setRadiusMetres] = useState(zone.radiusMetres);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const nameError = useMemo(() => {
    if (name.trim() === '') {
      return 'Zone name is required';
    }
    return null;
  }, [name]);

  const isDirty = name !== baselineName || radiusMetres !== baselineRadius;

  const canSave = isDirty && nameError === null;

  const save = useCallback(async () => {
    if (!isDirty) return;

    const data: UpdateWatchZoneRequest = {};
    const mutableData = data as Record<string, unknown>;
    if (name !== baselineName) {
      mutableData['name'] = name;
    }
    if (radiusMetres !== baselineRadius) {
      mutableData['radiusMetres'] = radiusMetres;
    }

    setIsSaving(true);
    setError(null);
    try {
      const updated = await repository.updateZone(zone.id, data);
      setBaselineName(updated.name);
      setBaselineRadius(updated.radiusMetres);
      setName(updated.name);
      setRadiusMetres(updated.radiusMetres);
      setIsSaving(false);
    } catch (err: unknown) {
      setIsSaving(false);
      setError(extractErrorMessage(err));
    }
  }, [isDirty, name, radiusMetres, baselineName, baselineRadius, zone.id, repository]);

  return {
    name,
    setName,
    radiusMetres,
    setRadiusMetres,
    isDirty,
    isSaving,
    error,
    nameError,
    canSave,
    save,
  };
}
