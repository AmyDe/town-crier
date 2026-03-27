import type { UserProfile, WatchZoneSummary, CreateWatchZoneRequest } from '../../../../domain/types';
import type { OnboardingPort } from '../../../../domain/ports/onboarding-port';

export class SpyOnboardingPort implements OnboardingPort {
  createProfileCalls = 0;
  createProfileResult: UserProfile = {
    userId: 'user-1',
    postcode: null,
    pushEnabled: false,
    tier: 'Free',
  };
  createProfileError: Error | null = null;

  async createProfile(): Promise<UserProfile> {
    this.createProfileCalls += 1;
    if (this.createProfileError) {
      throw this.createProfileError;
    }
    return this.createProfileResult;
  }

  createWatchZoneCalls: CreateWatchZoneRequest[] = [];
  createWatchZoneResult: WatchZoneSummary = {
    id: 'zone-1' as WatchZoneSummary['id'],
    name: 'Home',
    latitude: 51.5074,
    longitude: -0.1278,
    radiusMetres: 1000,
    authorityId: 0 as WatchZoneSummary['authorityId'],
  };
  createWatchZoneError: Error | null = null;

  async createWatchZone(request: CreateWatchZoneRequest): Promise<WatchZoneSummary> {
    this.createWatchZoneCalls.push(request);
    if (this.createWatchZoneError) {
      throw this.createWatchZoneError;
    }
    return this.createWatchZoneResult;
  }
}
