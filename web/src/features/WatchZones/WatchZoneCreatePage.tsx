import { Link } from 'react-router';
import type { WatchZoneRepository } from '../../domain/ports/watch-zone-repository';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import type { SubscriptionTier } from '../../domain/types';
import { useCreateWatchZone } from './useCreateWatchZone';
import { useBoundaryDrawing } from '../../hooks/useBoundaryDrawing';
import { PostcodeInput } from '../../components/PostcodeInput/PostcodeInput';
import { RadiusPicker } from '../../components/RadiusPicker/RadiusPicker';
import { LargeRadiusWarning } from '../../components/LargeRadiusWarning/LargeRadiusWarning';
import { ConfirmMap } from '../../components/ConfirmMap/ConfirmMap';
import { ShapeModeToggle } from './ShapeModeToggle';
import { BoundaryMap } from '../../components/BoundaryMap/BoundaryMap';
import { CustomShapeUpsell } from '../../components/CustomShapeUpsell/CustomShapeUpsell';
import { appStoreUrl } from '../../config/links';
import styles from './WatchZoneCreatePage.module.css';

interface Props {
  repository: WatchZoneRepository;
  geocodingPort: GeocodingPort;
  navigate: (path: string) => void;
  /**
   * The current user's subscription tier. Only Personal/Pro can draw a
   * custom shape (GH#1031) — Free sees the radius picker plus an upsell,
   * with no drawing affordance. Optional so legacy call sites keep
   * compiling; defaults to Free.
   */
  tier?: SubscriptionTier;
}

export function WatchZoneCreatePage({ repository, geocodingPort, navigate, tier = 'Free' }: Props) {
  const {
    step,
    name,
    postcode,
    coordinates,
    radiusMetres,
    shapeMode,
    isSaving,
    error,
    setGeocode,
    setName,
    setRadiusMetres,
    setShapeMode,
    confirmDetails,
    save,
  } = useCreateWatchZone(repository, navigate);

  const drawing = useBoundaryDrawing();
  const canDrawCustomShape = tier !== 'Free';
  const isCustomShape = canDrawCustomShape && shapeMode === 'custom';

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Create Watch Zone</h1>
        <Link to="/watch-zones" className={styles.cancelLink}>
          Cancel
        </Link>
      </div>

      {step === 'postcode' && (
        <section className={styles.section}>
          <p className={styles.instruction}>
            Enter a postcode to set the centre of your watch zone.
          </p>
          <PostcodeInput geocodingPort={geocodingPort} onGeocode={setGeocode} />
        </section>
      )}

      {step === 'details' && (
        <section className={styles.section}>
          <div className={styles.field}>
            <label htmlFor="zone-name" className={styles.label}>
              Zone name
            </label>
            <input
              id="zone-name"
              type="text"
              className={styles.input}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Home, Office"
            />
          </div>

          {canDrawCustomShape && <ShapeModeToggle mode={shapeMode} onSelect={setShapeMode} />}

          {isCustomShape ? (
            coordinates && (
              <BoundaryMap
                centre={coordinates}
                vertices={drawing.vertices}
                isClosed={drawing.isClosed}
                onAddVertex={drawing.addVertex}
                onMoveVertex={drawing.moveVertex}
                onCloseRing={drawing.closeRing}
                onUndo={drawing.undo}
                onReset={drawing.reset}
              />
            )
          ) : (
            <>
              <RadiusPicker selectedMetres={radiusMetres} onSelect={setRadiusMetres} />
              <LargeRadiusWarning radiusMetres={radiusMetres} />
            </>
          )}

          {!canDrawCustomShape && (
            <CustomShapeUpsell
              upgradeHref={appStoreUrl('web-watch-zone-create-custom-shape')}
            />
          )}

          {error && (
            <p className={styles.error} role="alert">
              {error}
            </p>
          )}

          <div className={styles.actions}>
            <button
              type="button"
              className={styles.saveButton}
              onClick={() => confirmDetails(drawing.boundary)}
            >
              Next
            </button>
          </div>
        </section>
      )}

      {step === 'confirm' && (
        <section className={styles.section}>
          {isCustomShape
            ? coordinates && (
                <BoundaryMap
                  centre={coordinates}
                  vertices={drawing.vertices}
                  isClosed={drawing.isClosed}
                  onAddVertex={drawing.addVertex}
                  onMoveVertex={drawing.moveVertex}
                  onCloseRing={drawing.closeRing}
                  onUndo={drawing.undo}
                  onReset={drawing.reset}
                />
              )
            : coordinates && (
                <ConfirmMap
                  latitude={coordinates.latitude}
                  longitude={coordinates.longitude}
                  radiusMetres={radiusMetres}
                />
              )}
          <div className={styles.confirmDetails}>
            <div className={styles.confirmRow}>
              <span className={styles.confirmLabel}>Postcode</span>
              <span className={styles.confirmValue}>{postcode}</span>
            </div>
            <div className={styles.confirmRow}>
              <span className={styles.confirmLabel}>Name</span>
              <span className={styles.confirmValue}>{name}</span>
            </div>
            <div className={styles.confirmRow}>
              <span className={styles.confirmLabel}>Shape</span>
              <span className={styles.confirmValue}>
                {isCustomShape ? 'Custom shape' : 'Circle'}
              </span>
            </div>
            {!isCustomShape && (
              <div className={styles.confirmRow}>
                <span className={styles.confirmLabel}>Radius</span>
                <span className={styles.confirmValue}>
                  {radiusMetres >= 1000 ? `${radiusMetres / 1000} km` : `${radiusMetres} m`}
                </span>
              </div>
            )}
          </div>
          {error && (
            <p className={styles.error} role="alert">
              {error}
            </p>
          )}
          <div className={styles.actions}>
            <button
              type="button"
              className={styles.saveButton}
              onClick={() => save(drawing.boundary)}
              disabled={isSaving}
            >
              {isSaving ? 'Saving...' : 'Confirm'}
            </button>
          </div>
        </section>
      )}

      {step === 'postcode' && error && (
        <p className={styles.error} role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
