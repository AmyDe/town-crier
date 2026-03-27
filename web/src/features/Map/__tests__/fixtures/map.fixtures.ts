import type { WatchZoneSummary, PlanningApplication } from '../../../../domain/types';
import { asWatchZoneId, asAuthorityId, asApplicationUid } from '../../../../domain/types';

export function aWatchZone(overrides?: Partial<WatchZoneSummary>): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-1'),
    name: 'Home',
    latitude: 52.2053,
    longitude: 0.1218,
    radiusMetres: 2000,
    authorityId: asAuthorityId(1),
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
    ...overrides,
  };
}

export function anApplication(overrides?: Partial<PlanningApplication>): PlanningApplication {
  return {
    uid: asApplicationUid('APP-001'),
    name: '2026/0042/FUL',
    address: '12 Mill Road, Cambridge, CB1 2AD',
    postcode: 'CB1 2AD',
    description: 'Erection of two-storey rear extension',
    appType: 'Full Planning',
    appState: 'Undecided',
    appSize: null,
    areaName: 'Cambridge City Council',
    areaId: asAuthorityId(1),
    startDate: '2026-01-15',
    decidedDate: null,
    consultedDate: '2026-02-01',
    latitude: 52.2053,
    longitude: 0.1218,
    url: 'https://council.example.com/planning/APP-001',
    link: 'https://planit.org.uk/planapplic/APP-001',
    lastDifferent: '2026-01-20',
    ...overrides,
  };
}

export function aSecondApplication(overrides?: Partial<PlanningApplication>): PlanningApplication {
  return {
    uid: asApplicationUid('APP-002'),
    name: '2026/0099/LBC',
    address: '45 High Street, Cambridge, CB2 1LA',
    postcode: 'CB2 1LA',
    description: 'Change of use from retail to residential',
    appType: 'Listed Building Consent',
    appState: 'Approved',
    appSize: null,
    areaName: 'Cambridge City Council',
    areaId: asAuthorityId(1),
    startDate: '2026-02-01',
    decidedDate: '2026-03-10',
    consultedDate: null,
    latitude: 52.2070,
    longitude: 0.1200,
    url: 'https://council.example.com/planning/APP-002',
    link: 'https://planit.org.uk/planapplic/APP-002',
    lastDifferent: '2026-03-10',
    ...overrides,
  };
}
