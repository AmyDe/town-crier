import type { DesignationContext } from '../../../../domain/types';

export function noDesignations(
  overrides?: Partial<DesignationContext>,
): DesignationContext {
  return {
    isWithinConservationArea: false,
    conservationAreaName: null,
    isWithinListedBuildingCurtilage: false,
    listedBuildingGrade: null,
    isWithinArticle4Area: false,
    ...overrides,
  };
}

export function conservationAreaDesignation(
  overrides?: Partial<DesignationContext>,
): DesignationContext {
  return {
    isWithinConservationArea: true,
    conservationAreaName: 'Mill Road Conservation Area',
    isWithinListedBuildingCurtilage: false,
    listedBuildingGrade: null,
    isWithinArticle4Area: false,
    ...overrides,
  };
}

export function allDesignations(
  overrides?: Partial<DesignationContext>,
): DesignationContext {
  return {
    isWithinConservationArea: true,
    conservationAreaName: 'Historic Town Centre',
    isWithinListedBuildingCurtilage: true,
    listedBuildingGrade: 'Grade II',
    isWithinArticle4Area: true,
    ...overrides,
  };
}
