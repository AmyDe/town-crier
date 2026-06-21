import { describe, it, expect } from 'vitest';
import {
  QUALIFYING_AREA_TYPES,
  isQualifyingAreaType,
  filterQualifyingAuthorities,
} from '../area-types.mjs';

describe('QUALIFYING_AREA_TYPES', () => {
  it('contains exactly the nine local-planning-authority area types', () => {
    expect([...QUALIFYING_AREA_TYPES].sort()).toEqual(
      [
        'Council District',
        'English District',
        'English Unitary Authority',
        'London Borough',
        'Metropolitan Borough',
        'National Park',
        'Northern Ireland District',
        'Scottish Council',
        'Welsh Principal Area',
      ].sort(),
    );
  });
});

describe('isQualifyingAreaType', () => {
  it.each([
    'English District',
    'English Unitary Authority',
    'Council District',
    'Metropolitan Borough',
    'London Borough',
    'Scottish Council',
    'Welsh Principal Area',
    'National Park',
    'Northern Ireland District',
  ])('includes %s', (areaType) => {
    expect(isQualifyingAreaType(areaType)).toBe(true);
  });

  it.each([
    'English Region',
    'UK Nation',
    'English County',
    'Metropolitan County',
    'Cross Border Area',
    'Other Planning Entity',
    'Crown Dependency',
    'Crown Dependencies',
    'Combined Planning Authority',
  ])('excludes %s', (areaType) => {
    expect(isQualifyingAreaType(areaType)).toBe(false);
  });
});

describe('filterQualifyingAuthorities', () => {
  it('keeps only authorities whose areaType qualifies', () => {
    const input = [
      { id: 1, name: 'Adur', areaType: 'English District' },
      { id: 2, name: 'West Sussex', areaType: 'English County' },
      { id: 3, name: 'Cardiff', areaType: 'Welsh Principal Area' },
      { id: 4, name: 'Greater London', areaType: 'English Region' },
    ];

    expect(filterQualifyingAuthorities(input)).toEqual([
      { id: 1, name: 'Adur', areaType: 'English District' },
      { id: 3, name: 'Cardiff', areaType: 'Welsh Principal Area' },
    ]);
  });
});
