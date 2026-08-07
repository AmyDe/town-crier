import { Navigate } from 'react-router';
import type { OnboardingPort } from '../../domain/ports/onboarding-port';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import { ConfirmMap } from '../../components/ConfirmMap/ConfirmMap';
import { PostcodeInput } from '../../components/PostcodeInput/PostcodeInput';
import { RadiusPicker } from '../../components/RadiusPicker/RadiusPicker';
import { BoundaryMap } from '../../components/BoundaryMap/BoundaryMap';
import { CustomShapeUpsell } from '../../components/CustomShapeUpsell/CustomShapeUpsell';
import { useBoundaryDrawing } from '../../hooks/useBoundaryDrawing';
import { appStoreUrl } from '../../config/links';
import { useOnboarding } from './useOnboarding';
import styles from './OnboardingPage.module.css';

interface Props {
  onboardingPort: OnboardingPort;
  geocodingPort: GeocodingPort;
}

export function OnboardingPage({ onboardingPort, geocodingPort }: Props) {
  const {
    step,
    geocode,
    postcode,
    radiusMetres,
    tier,
    isSubmitting,
    error,
    isComplete,
    start,
    handleGeocode,
    selectRadius,
    confirmRadius,
    confirmDrawing,
    finish,
  } = useOnboarding(onboardingPort);

  const drawing = useBoundaryDrawing();

  if (isComplete) {
    return <Navigate to="/dashboard" replace />;
  }

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        {step === 'welcome' && (
          <>
            <h1 className={styles.heading}>Welcome to Town Crier</h1>
            <p className={styles.description}>
              Stay informed about planning applications near you. Let&apos;s set up your first watch zone.
            </p>
            <button
              type="button"
              className={styles.primaryButton}
              onClick={start}
            >
              Get Started
            </button>
          </>
        )}

        {step === 'postcode' && (
          <>
            <h2 className={styles.stepLabel}>Enter your postcode</h2>
            <PostcodeInput
              geocodingPort={geocodingPort}
              onGeocode={handleGeocode}
            />
          </>
        )}

        {step === 'radius' && (
          <>
            <h2 className={styles.stepLabel}>Choose your radius</h2>
            <RadiusPicker
              selectedMetres={radiusMetres}
              onSelect={selectRadius}
            />
            {tier === 'Free' && (
              <CustomShapeUpsell
                upgradeHref={appStoreUrl('web-onboarding-custom-shape')}
              />
            )}
            <button
              type="button"
              className={styles.primaryButton}
              onClick={confirmRadius}
            >
              Next
            </button>
          </>
        )}

        {step === 'drawing' && geocode && (
          <>
            <h2 className={styles.stepLabel}>Draw your watch zone</h2>
            <BoundaryMap
              centre={geocode}
              vertices={drawing.vertices}
              isClosed={drawing.isClosed}
              onAddVertex={drawing.addVertex}
              onMoveVertex={drawing.moveVertex}
              onCloseRing={drawing.closeRing}
              onUndo={drawing.undo}
              onReset={drawing.reset}
            />
            <button
              type="button"
              className={styles.primaryButton}
              onClick={() => confirmDrawing(drawing.boundary)}
            >
              Next
            </button>
            {error && (
              <p className={styles.error} role="alert">
                {error}
              </p>
            )}
          </>
        )}

        {step === 'confirm' && (
          <>
            <h2 className={styles.stepLabel}>Confirm your watch zone</h2>
            {geocode && (
              drawing.boundary ? (
                <BoundaryMap
                  centre={geocode}
                  vertices={drawing.vertices}
                  isClosed={drawing.isClosed}
                  onAddVertex={drawing.addVertex}
                  onMoveVertex={drawing.moveVertex}
                  onCloseRing={drawing.closeRing}
                  onUndo={drawing.undo}
                  onReset={drawing.reset}
                />
              ) : (
                <ConfirmMap
                  latitude={geocode.latitude}
                  longitude={geocode.longitude}
                  radiusMetres={radiusMetres}
                />
              )
            )}
            <div className={styles.confirmDetails}>
              <div className={styles.confirmRow}>
                <span className={styles.confirmLabel}>Postcode</span>
                <span className={styles.confirmValue}>{postcode}</span>
              </div>
              <div className={styles.confirmRow}>
                <span className={styles.confirmLabel}>
                  {drawing.boundary ? 'Shape' : 'Radius'}
                </span>
                <span className={styles.confirmValue}>
                  {drawing.boundary
                    ? 'Custom shape'
                    : radiusMetres >= 1000 ? `${radiusMetres / 1000} km` : `${radiusMetres} m`}
                </span>
              </div>
            </div>
            <button
              type="button"
              className={styles.primaryButton}
              onClick={() => finish(drawing.boundary)}
              disabled={isSubmitting}
            >
              {isSubmitting ? 'Setting up...' : 'Confirm'}
            </button>
            {error && (
              <p className={styles.error} role="alert">
                {error}
              </p>
            )}
          </>
        )}
      </div>
    </div>
  );
}
