import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, it, expect } from 'vitest';
import { slugify } from '../slug.mjs';

const FIXTURE_PATH = join(
  dirname(fileURLToPath(import.meta.url)),
  '..',
  'slug-fixtures.json',
);
const fixtures = JSON.parse(readFileSync(FIXTURE_PATH, 'utf-8'));

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

// Shared test-vector fixture. The SAME slug-fixtures.json is read by the Go
// parity test so the two slugify implementations can never drift. Every
// `expected` value is ground-truth: it is exactly what slug.mjs's slugify()
// returns for the corresponding `input` (verified by running slug.mjs).
describe('slugify parity fixture', () => {
  it('has at least one vector', () => {
    expect(fixtures.length).toBeGreaterThan(0);
  });

  it.each(fixtures)('slugify($input) === $expected', ({ input, expected }) => {
    expect(slugify(input)).toBe(expected);
  });
});
