import { useState, useCallback, useMemo } from 'react';
import type { WatchZoneSummary, UpdateWatchZoneRequest, WatchZoneBoundary } from '../../domain/types';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import type { ShapeMode } from './ShapeModeToggle';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

const MISSING_BOUNDARY_ERROR =
  'Draw a shape with at least 3 points, then close it by clicking the first point again.';

function shapeModeOf(boundary: WatchZoneBoundary | null): ShapeMode {
  return boundary ? 'custom' : 'circle';
}

/** Structural equality for a GeoJSON boundary — good enough here since
 * both sides always come from the same wire shape (no extra keys, no
 * `NaN`s), and a byte-for-byte compare is exactly what "has this shape
 * been redrawn" means. */
function boundariesEqual(a: WatchZoneBoundary | null, b: WatchZoneBoundary | null): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

export function useZoneEdit(
  repository: WatchZoneRepository,
  zone: WatchZoneSummary,
  currentBoundary?: WatchZoneBoundary | null,
) {
  const [baselineName, setBaselineName] = useState(zone.name);
  const [baselineRadius, setBaselineRadius] = useState(zone.radiusMetres);
  const [baselinePushEnabled, setBaselinePushEnabled] = useState(zone.pushEnabled);
  const [baselineEmailInstantEnabled, setBaselineEmailInstantEnabled] = useState(
    zone.emailInstantEnabled,
  );
  const [baselineBoundary, setBaselineBoundary] = useState<WatchZoneBoundary | null>(
    zone.boundary,
  );
  const [name, setName] = useState(zone.name);
  const [radiusMetres, setRadiusMetres] = useState(zone.radiusMetres);
  const [pushEnabled, setPushEnabled] = useState(zone.pushEnabled);
  const [emailInstantEnabled, setEmailInstantEnabled] = useState(zone.emailInstantEnabled);
  const [shapeMode, setShapeMode] = useState<ShapeMode>(() => shapeModeOf(zone.boundary));
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const nameError = useMemo(() => {
    if (name.trim() === '') {
      return 'Zone name is required';
    }
    return null;
  }, [name]);

  // The boundary actually in play right now: `null` in circle mode
  // regardless of what's being drawn, otherwise whatever the caller passed
  // (the live value from `useBoundaryDrawing`, or `null`/undefined while
  // nothing has been drawn yet).
  const effectiveBoundary = shapeMode === 'custom' ? (currentBoundary ?? null) : null;

  const boundaryError = useMemo(() => {
    if (shapeMode === 'custom' && effectiveBoundary === null) {
      return MISSING_BOUNDARY_ERROR;
    }
    return null;
  }, [shapeMode, effectiveBoundary]);

  const baselineShapeMode = shapeModeOf(baselineBoundary);
  const boundaryDirty =
    shapeMode !== baselineShapeMode ||
    (shapeMode === 'custom' && !boundariesEqual(effectiveBoundary, baselineBoundary));

  const isDirty =
    name !== baselineName ||
    radiusMetres !== baselineRadius ||
    pushEnabled !== baselinePushEnabled ||
    emailInstantEnabled !== baselineEmailInstantEnabled ||
    boundaryDirty;

  const canSave = isDirty && nameError === null && boundaryError === null;

  const save = useCallback(async () => {
    if (!isDirty || boundaryError !== null) return;

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
    if (boundaryDirty) {
      // Tri-state: omitted entirely above when unchanged, explicit `null`
      // to revert to a circle, or the new/redrawn shape.
      mutableData['boundary'] = effectiveBoundary;
    }

    setIsSaving(true);
    setError(null);
    try {
      const updated = await repository.updateZone(zone.id, data);
      setBaselineName(updated.name);
      setBaselineRadius(updated.radiusMetres);
      setBaselinePushEnabled(updated.pushEnabled);
      setBaselineEmailInstantEnabled(updated.emailInstantEnabled);
      setBaselineBoundary(updated.boundary);
      setName(updated.name);
      setRadiusMetres(updated.radiusMetres);
      setPushEnabled(updated.pushEnabled);
      setEmailInstantEnabled(updated.emailInstantEnabled);
      setShapeMode(shapeModeOf(updated.boundary));
      setIsSaving(false);
    } catch (err: unknown) {
      setIsSaving(false);
      setError(extractErrorMessage(err));
    }
  }, [
    isDirty,
    boundaryError,
    name,
    radiusMetres,
    pushEnabled,
    emailInstantEnabled,
    boundaryDirty,
    effectiveBoundary,
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
    shapeMode,
    setShapeMode,
    isDirty,
    isSaving,
    error,
    nameError,
    boundaryError,
    canSave,
    save,
  };
}
