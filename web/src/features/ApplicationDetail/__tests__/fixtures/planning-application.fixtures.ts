import type { PlanningApplication } from '../../../../domain/types';
import { asApplicationUid, asAuthorityId } from '../../../../domain/types';

export function fullApplication(
  overrides?: Partial<PlanningApplication>,
): PlanningApplication {
  return {
    uid: asApplicationUid('APP-001'),
    name: '2026/0042/FUL',
    address: '12 Mill Road, Cambridge, CB1 2AD',
    postcode: 'CB1 2AD',
    description: 'Erection of two-storey rear extension with associated landscaping',
    appType: 'Full Planning',
    appState: 'Undecided',
    appSize: null,
    areaName: 'Cambridge City Council',
    areaId: asAuthorityId(42),
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

export function approvedWithDecision(
  overrides?: Partial<PlanningApplication>,
): PlanningApplication {
  return {
    ...fullApplication(),
    uid: asApplicationUid('APP-002'),
    name: '2026/0099/LBC',
    appState: 'Approved',
    decidedDate: '2026-03-10',
    ...overrides,
  };
}

export function applicationWithoutCoordinates(
  overrides?: Partial<PlanningApplication>,
): PlanningApplication {
  return {
    ...fullApplication(),
    uid: asApplicationUid('APP-003'),
    latitude: null,
    longitude: null,
    ...overrides,
  };
}

export function applicationWithoutUrl(
  overrides?: Partial<PlanningApplication>,
): PlanningApplication {
  return {
    ...fullApplication(),
    uid: asApplicationUid('APP-004'),
    url: null,
    ...overrides,
  };
}
