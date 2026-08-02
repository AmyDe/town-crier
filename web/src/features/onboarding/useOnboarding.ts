import { useState, useCallback } from 'react';
import type { GeocodeResult, SubscriptionTier, WatchZoneBoundary } from '../../domain/types';
import type { OnboardingPort } from '../../domain/ports/onboarding-port';
import { extractErrorMessage } from '../../utils/extractErrorMessage';

export type OnboardingStep = 'welcome' | 'postcode' | 'radius' | 'drawing' | 'confirm';

const MISSING_BOUNDARY_ERROR =
  'Draw a shape with at least 3 points, then close it by clicking the first point again.';

export function useOnboarding(port: OnboardingPort) {
  const [step, setStep] = useState<OnboardingStep>('welcome');
  const [geocode, setGeocode] = useState<GeocodeResult | null>(null);
  const [postcode, setPostcode] = useState('');
  const [radiusMetres, setRadiusMetres] = useState(1000);
  /**
   * A brand-new onboarding user has no established subscription yet
   * (ConnectedOnboardingPage only reaches this flow when `fetchProfile()`
   * resolves `null`), so this always starts Free. `upgradeTier` exists so a
   * future purchase-completion callback can flip it mid-flow — nothing
   * calls it from real UI yet, since no billing/checkout route exists on
   * web (GH#1031 section 8). See `upgradeTier` below.
   */
  const [tier, setTier] = useState<SubscriptionTier>('Free');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isComplete, setIsComplete] = useState(false);

  const start = useCallback(() => {
    setStep('postcode');
  }, []);

  const handleGeocode = useCallback((result: GeocodeResult, enteredPostcode: string) => {
    setGeocode(result);
    setPostcode(enteredPostcode);
    setStep('radius');
  }, []);

  const selectRadius = useCallback((metres: number) => {
    setRadiusMetres(metres);
  }, []);

  const confirmRadius = useCallback(() => {
    setStep('confirm');
  }, []);

  const confirmDrawing = useCallback((boundary?: WatchZoneBoundary | null) => {
    if (!boundary) {
      setError(MISSING_BOUNDARY_ERROR);
      return;
    }
    setError(null);
    setStep('confirm');
  }, []);

  /**
   * Flips the onboarding tier mid-flow. There is no purchase/checkout UI on
   * web yet (GH#1031 section 8) — nothing calls this today — but it exists
   * so a future purchase-completion callback can hook in without
   * restarting onboarding. When the user upgrades away from Free while
   * sat on the radius step, swap straight to the drawing step so they land
   * on the boundary-drawing UI instead of losing their place.
   */
  const upgradeTier = useCallback((newTier: SubscriptionTier) => {
    setTier(newTier);
    if (newTier !== 'Free') {
      setStep((current) => (current === 'radius' ? 'drawing' : current));
    }
  }, []);

  const finish = useCallback(async (boundary?: WatchZoneBoundary | null) => {
    if (!geocode) return;

    setIsSubmitting(true);
    setError(null);
    try {
      await port.createProfile();
      await port.createWatchZone({
        name: 'Home',
        latitude: geocode.latitude,
        longitude: geocode.longitude,
        radiusMetres,
        ...(boundary ? { boundary } : {}),
      });
      setIsComplete(true);
    } catch (err) {
      setError(extractErrorMessage(err, 'Something went wrong'));
    } finally {
      setIsSubmitting(false);
    }
  }, [port, geocode, radiusMetres]);

  return {
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
    upgradeTier,
    finish,
  };
}
