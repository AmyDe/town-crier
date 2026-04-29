import { useState, useCallback, useMemo } from 'react';
import type { WatchZoneSummary, UpdateWatchZoneRequest } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

export function useZoneEdit(repository: WatchZoneRepository, zone: WatchZoneSummary) {
  const [baselineName, setBaselineName] = useState(zone.name);
  const [baselineRadius, setBaselineRadius] = useState(zone.radiusMetres);
  const [baselinePushEnabled, setBaselinePushEnabled] = useState(zone.pushEnabled);
  const [baselineEmailInstantEnabled, setBaselineEmailInstantEnabled] = useState(
    zone.emailInstantEnabled,
  );
  const [name, setName] = useState(zone.name);
  const [radiusMetres, setRadiusMetres] = useState(zone.radiusMetres);
  const [pushEnabled, setPushEnabled] = useState(zone.pushEnabled);
  const [emailInstantEnabled, setEmailInstantEnabled] = useState(zone.emailInstantEnabled);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const nameError = useMemo(() => {
    if (name.trim() === '') {
      return 'Zone name is required';
    }
    return null;
  }, [name]);

  const isDirty =
    name !== baselineName ||
    radiusMetres !== baselineRadius ||
    pushEnabled !== baselinePushEnabled ||
    emailInstantEnabled !== baselineEmailInstantEnabled;

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
    if (pushEnabled !== baselinePushEnabled) {
      mutableData['pushEnabled'] = pushEnabled;
    }
    if (emailInstantEnabled !== baselineEmailInstantEnabled) {
      mutableData['emailInstantEnabled'] = emailInstantEnabled;
    }

    setIsSaving(true);
    setError(null);
    try {
      const updated = await repository.updateZone(zone.id, data);
      setBaselineName(updated.name);
      setBaselineRadius(updated.radiusMetres);
      setBaselinePushEnabled(updated.pushEnabled);
      setBaselineEmailInstantEnabled(updated.emailInstantEnabled);
      setName(updated.name);
      setRadiusMetres(updated.radiusMetres);
      setPushEnabled(updated.pushEnabled);
      setEmailInstantEnabled(updated.emailInstantEnabled);
      setIsSaving(false);
    } catch (err: unknown) {
      setIsSaving(false);
      setError(extractErrorMessage(err));
    }
  }, [
    isDirty,
    name,
    radiusMetres,
    pushEnabled,
    emailInstantEnabled,
    baselineName,
    baselineRadius,
    baselinePushEnabled,
    baselineEmailInstantEnabled,
    zone.id,
    repository,
  ]);

  return {
    name,
    setName,
    radiusMetres,
    setRadiusMetres,
    pushEnabled,
    setPushEnabled,
    emailInstantEnabled,
    setEmailInstantEnabled,
    isDirty,
    isSaving,
    error,
    nameError,
    canSave,
    save,
  };
}
