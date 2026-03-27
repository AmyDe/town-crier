import type { PlanningApplicationSummary, SearchResult } from '../../../../domain/types';
import { asApplicationUid } from '../../../../domain/types';

export function anApplication(
  overrides?: Partial<PlanningApplicationSummary>,
): PlanningApplicationSummary {
  return {
    uid: asApplicationUid('APP-001'),
    name: '2026/0042/FUL',
    address: '12 Mill Road, Cambridge, CB1 2AD',
    postcode: 'CB1 2AD',
    description: 'Erection of two-storey rear extension',
    appType: 'Full Planning',
    appState: 'Undecided',
    areaName: 'Cambridge City Council',
    startDate: '2026-01-15',
    url: 'https://planit.org.uk/planapplic/APP-001',
    ...overrides,
  };
}

export function aSecondApplication(
  overrides?: Partial<PlanningApplicationSummary>,
): PlanningApplicationSummary {
  return {
    uid: asApplicationUid('APP-002'),
    name: '2026/0099/LBC',
    address: '45 High Street, Cambridge, CB2 1LA',
    postcode: 'CB2 1LA',
    description: 'Change of use from retail to residential',
    appType: 'Listed Building Consent',
    appState: 'Approved',
    areaName: 'Cambridge City Council',
    startDate: '2026-02-20',
    url: null,
    ...overrides,
  };
}

export function searchResultPage(
  items: PlanningApplicationSummary[] = [anApplication(), aSecondApplication()],
  total: number = 2,
  page: number = 1,
): SearchResult {
  return { applications: items, total, page };
}
