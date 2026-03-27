import type { PlanningApplicationSummary, WatchZoneSummary } from '../../../../domain/types';
import { asApplicationUid, asAuthorityId, asWatchZoneId } from '../../../../domain/types';

export function cambridgeZone(
  overrides?: Partial<WatchZoneSummary>,
): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-001'),
    name: 'Home - Cambridge',
    latitude: 52.2053,
    longitude: 0.1218,
    radiusMetres: 1000,
    authorityId: asAuthorityId(42),
    ...overrides,
  };
}

export function oxfordZone(
  overrides?: Partial<WatchZoneSummary>,
): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-002'),
    name: 'Office - Oxford',
    latitude: 51.752,
    longitude: -1.2577,
    radiusMetres: 500,
    authorityId: asAuthorityId(99),
    ...overrides,
  };
}

export function recentApplication(
  overrides?: Partial<PlanningApplicationSummary>,
): PlanningApplicationSummary {
  return {
    uid: asApplicationUid('APP-101'),
    name: '2026/0042/FUL',
    address: '12 Mill Road, Cambridge, CB1 2AD',
    postcode: 'CB1 2AD',
    description: 'Erection of two-storey rear extension',
    appType: 'Full Planning',
    appState: 'Undecided',
    areaName: 'Cambridge City Council',
    startDate: '2026-03-15',
    url: null,
    ...overrides,
  };
}

export function anotherRecentApplication(
  overrides?: Partial<PlanningApplicationSummary>,
): PlanningApplicationSummary {
  return {
    uid: asApplicationUid('APP-102'),
    name: '2026/0088/LBC',
    address: '45 High Street, Oxford, OX1 4AS',
    postcode: 'OX1 4AS',
    description: 'Change of use from retail to residential',
    appType: 'Listed Building Consent',
    appState: 'Approved',
    areaName: 'Oxford City Council',
    startDate: '2026-03-10',
    url: null,
    ...overrides,
  };
}
