import { useState, useEffect, useMemo } from 'react';
import { Navigate } from 'react-router-dom';
import { useApiClient } from '../../api/useApiClient';
import { userProfileApi } from '../../api/userProfile';
import { watchZonesApi } from '../../api/watchZones';
import { geocodingApi } from '../../api/geocoding';
import { useProfileRepository } from '../../auth/profile-context';
import type { OnboardingPort } from '../../domain/ports/onboarding-port';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import { OnboardingPage } from './OnboardingPage';

export function ConnectedOnboardingPage() {
  const client = useApiClient();
  const repository = useProfileRepository();
  const [hasProfile, setHasProfile] = useState<boolean | null>(null);

  useEffect(() => {
    let cancelled = false;
    repository.fetchProfile().then((profile) => {
      if (!cancelled) setHasProfile(profile !== null);
    });
    return () => { cancelled = true; };
  }, [repository]);

  const onboardingPort: OnboardingPort = useMemo(() => {
    const profile = userProfileApi(client);
    const zones = watchZonesApi(client);
    return {
      createProfile: () => profile.create(),
      createWatchZone: (request) => zones.create(request),
    };
  }, [client]);

  const geocodingPort: GeocodingPort = useMemo(() => {
    const geo = geocodingApi(client);
    return { geocode: (postcode) => geo.geocode(postcode) };
  }, [client]);

  if (hasProfile === null) return null;
  if (hasProfile) return <Navigate to="/dashboard" replace />;

  return (
    <OnboardingPage
      onboardingPort={onboardingPort}
      geocodingPort={geocodingPort}
    />
  );
}
