import type {
  WatchZoneBoundary,
  WatchZoneSummary,
  ZoneNotificationPreferences,
} from '../../../../domain/types';
import { asWatchZoneId } from '../../../../domain/types';

export function aWatchZone(overrides?: Partial<WatchZoneSummary>): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-1'),
    name: 'Home',
    latitude: 52.2053,
    longitude: 0.1218,
    radiusMetres: 2000,
    pushEnabled: true,
    emailInstantEnabled: true,
    paused: false,
    boundary: null,
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
    pushEnabled: true,
    emailInstantEnabled: true,
    paused: false,
    boundary: null,
    ...overrides,
  };
}

/**
 * A small closed square ring, [longitude, latitude] tuple order — matches
 * the wire shape `WatchZoneBoundary` mirrors. First and last vertex are
 * identical, per the GeoJSON closed-ring convention.
 */
export function aWatchZoneBoundary(overrides?: Partial<WatchZoneBoundary>): WatchZoneBoundary {
  return {
    type: 'Polygon',
    coordinates: [
      [
        [0.1, 52.2],
        [0.12, 52.2],
        [0.12, 52.21],
        [0.1, 52.21],
        [0.1, 52.2],
      ],
    ],
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
