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

/** A small closed square ring in GeoJSON [longitude, latitude] order. */
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

/** A 0.1 degree square at 52N, about 76 km2: over the large-zone threshold. */
export function aLargeWatchZoneBoundary(): WatchZoneBoundary {
  return {
    type: 'Polygon',
    coordinates: [
      [
        [-1.05, 51.95],
        [-0.95, 51.95],
        [-0.95, 52.05],
        [-1.05, 52.05],
        [-1.05, 51.95],
      ],
    ],
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
