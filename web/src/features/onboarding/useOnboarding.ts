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
  // New onboarding users have no subscription yet, so the tier starts Free.
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

  // Leaving Free while on the radius step jumps to the drawing step.
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
