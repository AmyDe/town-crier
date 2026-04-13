import type { WatchZoneSummary } from '../../../../domain/types';
import { asWatchZoneId, asAuthorityId } from '../../../../domain/types';

export function cambridgeZone(
  overrides?: Partial<WatchZoneSummary>,
): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-cam-001'),
    name: 'Home - Cambridge',
    latitude: 52.2053,
    longitude: 0.1218,
    radiusMetres: 1000,
    authorityId: asAuthorityId(101),
    ...overrides,
  };
}

export function oxfordZone(
  overrides?: Partial<WatchZoneSummary>,
): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-oxf-001'),
    name: 'Office - Oxford',
    latitude: 51.752,
    longitude: -1.2577,
    radiusMetres: 500,
    authorityId: asAuthorityId(202),
    ...overrides,
  };
}
