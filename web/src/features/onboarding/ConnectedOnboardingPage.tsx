import { useMemo } from 'react';
import { useApiClient } from '../../api/useApiClient';
import { userProfileApi } from '../../api/userProfile';
import { watchZonesApi } from '../../api/watchZones';
import { geocodingApi } from '../../api/geocoding';
import type { OnboardingPort } from '../../domain/ports/onboarding-port';
import type { GeocodingPort } from '../../domain/ports/geocoding-port';
import { OnboardingPage } from './OnboardingPage';

export function ConnectedOnboardingPage() {
  const client = useApiClient();

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

  return (
    <OnboardingPage
      onboardingPort={onboardingPort}
      geocodingPort={geocodingPort}
    />
  );
}
