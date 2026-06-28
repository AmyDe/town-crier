import type {
  WatchZoneSummary,
  PlanningApplication,
  MapCluster,
  ClusterMember,
} from '../../../../domain/types';
import { asWatchZoneId, asAuthorityId, asApplicationUid } from '../../../../domain/types';

export function aZone(overrides?: Partial<WatchZoneSummary>): WatchZoneSummary {
  return {
    id: asWatchZoneId('zone-001'),
    name: 'Cambridge City Council',
    latitude: 52.2053,
    longitude: 0.1218,
    radiusMetres: 1000,
    authorityId: asAuthorityId(1),
    pushEnabled: true,
    emailInstantEnabled: false,
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
    pushEnabled: true,
    emailInstantEnabled: false,
    ...overrides,
  };
}

export function anApplication(overrides?: Partial<PlanningApplication>): PlanningApplication {
  return {
    name: '22/1234/FUL',
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
    longitude: 0.134,
    latitude: 52.199,
    url: null,
    link: null,
    lastDifferent: '2026-01-10',
    latestUnreadEvent: null,
    ...overrides,
  };
}

/** A multi-member cell — renders as an amber count bubble; `member` is null. */
export function aBubbleCluster(overrides?: Partial<MapCluster>): MapCluster {
  return {
    latitude: 52.2,
    longitude: 0.12,
    count: 7,
    statusCounts: { Undecided: 4, Permitted: 3 },
    member: null,
    ...overrides,
  };
}

/** A single-member cell — renders as a status-coloured pin carrying its member. */
export function aSinglePinCluster(overrides?: Partial<MapCluster>): MapCluster {
  const member: ClusterMember = { authority: '1', name: '22/1234/FUL' };
  return {
    latitude: 52.21,
    longitude: 0.13,
    count: 1,
    statusCounts: { Permitted: 1 },
    member,
    ...overrides,
  };
}
