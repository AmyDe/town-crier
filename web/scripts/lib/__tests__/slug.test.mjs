import { describe, it, expect } from 'vitest';
import { slugify } from '../slug.mjs';

describe('slugify', () => {
  it('lowercases and hyphenates a multi-word authority name', () => {
    expect(slugify('Basingstoke and Deane')).toBe('basingstoke-and-deane');
  });

  it('drops commas and other punctuation', () => {
    expect(slugify('Bristol, City of')).toBe('bristol-city-of');
  });

  it('strips apostrophes rather than turning them into hyphens', () => {
    expect(slugify("King's Lynn and West Norfolk")).toBe(
      'kings-lynn-and-west-norfolk',
    );
  });

  it('expands ampersands to "and"', () => {
    expect(slugify('Hammersmith & Fulham')).toBe('hammersmith-and-fulham');
  });

  it('collapses runs of whitespace and trims edges', () => {
    expect(slugify('  Test   Name  ')).toBe('test-name');
  });

  it('collapses a slash in a bilingual name to a single hyphen', () => {
    expect(slugify('Bro Morgannwg / Vale of Glamorgan')).toBe(
      'bro-morgannwg-vale-of-glamorgan',
    );
  });
});
