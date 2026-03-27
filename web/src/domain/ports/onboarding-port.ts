import type { UserProfile, WatchZoneSummary, CreateWatchZoneRequest } from '../types';

export interface OnboardingPort {
  createProfile(): Promise<UserProfile>;
  createWatchZone(request: CreateWatchZoneRequest): Promise<WatchZoneSummary>;
}
