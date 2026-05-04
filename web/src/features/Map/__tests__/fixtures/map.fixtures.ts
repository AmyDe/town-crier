import type { WatchZoneSummary, PlanningApplication, SavedApplication } from '../../../../domain/types';
import { asWatchZoneId, asAuthorityId, asApplicationUid } from '../../../../domain/types';

export function aZone(overrides?: Partial<WatchZoneSummary>): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-001'),
    name: 'Cambridge City Council',
    latitude: 52.2053,
    longitude: 0.1218,
    radiusMetres: 1000,
    authorityId: asAuthorityId(1),
    ...overrides,
  };
}

export function aSecondZone(overrides?: Partial<WatchZoneSummary>): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-002'),
    name: 'Westminster City Council',
    latitude: 51.4975,
    longitude: -0.1357,
    radiusMetres: 500,
    authorityId: asAuthorityId(2),
    ...overrides,
  };
}

export function anApplication(overrides?: Partial<PlanningApplication>): PlanningApplication {
  return {
    name: 'Application 1',
    uid: asApplicationUid('app-001'),
    areaName: 'Cambridge City Council',
    areaId: asAuthorityId(1),
    address: '12 Mill Road, Cambridge',
    postcode: 'CB1 2AD',
    description: 'Erection of two-storey rear extension',
    appType: 'Full Planning',
    appState: 'Undecided',
    appSize: null,
    startDate: '2026-01-15',
    decidedDate: null,
    consultedDate: null,
    longitude: 0.1340,
    latitude: 52.1990,
    url: null,
    link: null,
    lastDifferent: '2026-01-10',
    latestUnreadEvent: null,
    ...overrides,
  };
}

export function aSecondApplication(overrides?: Partial<PlanningApplication>): PlanningApplication {
  return {
    name: 'Application 2',
    uid: asApplicationUid('app-002'),
    areaName: 'Westminster City Council',
    areaId: asAuthorityId(2),
    address: '45 High Street, London',
    postcode: 'SW1A 1AA',
    description: 'Change of use from retail to residential',
    appType: 'Change of Use',
    appState: 'Permitted',
    appSize: null,
    startDate: '2026-02-01',
    decidedDate: '2026-03-01',
    consultedDate: null,
    longitude: -0.1357,
    latitude: 51.5014,
    url: null,
    link: null,
    lastDifferent: '2026-02-15',
    latestUnreadEvent: null,
    ...overrides,
  };
}

export function aSavedApplication(overrides?: Partial<SavedApplication>): SavedApplication {
  return {
    applicationUid: asApplicationUid('app-001'),
    savedAt: '2026-03-15T10:00:00Z',
    application: {
      uid: asApplicationUid('app-001'),
      name: 'Application 1',
      address: '12 Mill Road, Cambridge',
      postcode: 'CB1 2AD',
      description: 'Erection of two-storey rear extension',
      appType: 'Full Planning',
      appState: 'Undecided',
      areaName: 'Cambridge City Council',
      startDate: '2026-01-15',
      url: null,
      latestUnreadEvent: null,
    },
    ...overrides,
  };
}
