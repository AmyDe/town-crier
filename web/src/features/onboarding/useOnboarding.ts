import { useState, useCallback } from 'react';
import type { GeocodeResult } from '../../domain/types';
import type { OnboardingPort } from '../../domain/ports/onboarding-port';

export type OnboardingStep = 'welcome' | 'postcode' | 'radius' | 'confirm';

export function useOnboarding(port: OnboardingPort) {
  const [step, setStep] = useState<OnboardingStep>('welcome');
  const [geocode, setGeocode] = useState<GeocodeResult | null>(null);
  const [radiusMetres, setRadiusMetres] = useState(1000);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isComplete, setIsComplete] = useState(false);

  const start = useCallback(() => {
    setStep('postcode');
  }, []);

  const handleGeocode = useCallback((result: GeocodeResult) => {
    setGeocode(result);
    setStep('radius');
  }, []);

  const selectRadius = useCallback((metres: number) => {
    setRadiusMetres(metres);
  }, []);

  const confirmRadius = useCallback(() => {
    setStep('confirm');
  }, []);

  const finish = useCallback(async () => {
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
      });
      setIsComplete(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong');
    } finally {
      setIsSubmitting(false);
    }
  }, [port, geocode, radiusMetres]);

  return {
    step,
    geocode,
    radiusMetres,
    isSubmitting,
    error,
    isComplete,
    start,
    handleGeocode,
    selectRadius,
    confirmRadius,
    finish,
  };
}
