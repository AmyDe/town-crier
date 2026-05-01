import type {
  WatchZoneSummary,
  ZoneNotificationPreferences,
} from '../../../../domain/types';
import { asWatchZoneId, asAuthorityId } from '../../../../domain/types';

export function aWatchZone(overrides?: Partial<WatchZoneSummary>): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-1'),
    name: 'Home',
    latitude: 52.2053,
    longitude: 0.1218,
    radiusMetres: 2000,
    authorityId: asAuthorityId(1),
    pushEnabled: true,
    emailInstantEnabled: true,
    ...overrides,
  };
}

export function aSecondWatchZone(overrides?: Partial<WatchZoneSummary>): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-2'),
    name: 'Office',
    latitude: 51.5074,
    longitude: -0.1278,
    radiusMetres: 5000,
    authorityId: asAuthorityId(2),
    pushEnabled: true,
    emailInstantEnabled: true,
    ...overrides,
  };
}

export function zonePreferences(
  overrides?: Partial<ZoneNotificationPreferences>,
): ZoneNotificationPreferences {
  return {
    zoneId: asWatchZoneId('zone-1'),
    newApplicationPush: true,
    newApplicationEmail: true,
    decisionPush: true,
    decisionEmail: true,
    ...overrides,
  };
}
