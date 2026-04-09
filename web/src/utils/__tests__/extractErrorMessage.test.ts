import { describe, it, expect } from 'vitest';
import { extractErrorMessage } from '../extractErrorMessage';

describe('extractErrorMessage', () => {
  it('returns the message when given an Error instance', () => {
    const err = new Error('Something broke');

    const result = extractErrorMessage(err);

    expect(result).toBe('Something broke');
  });

  it('returns the default fallback when given a non-Error value', () => {
    const result = extractErrorMessage('a raw string');

    expect(result).toBe('An error occurred');
  });

  it('returns a custom fallback when given a non-Error value and a fallback', () => {
    const result = extractErrorMessage(42, 'Custom fallback');

    expect(result).toBe('Custom fallback');
  });

  it('returns the default fallback for null', () => {
    const result = extractErrorMessage(null);

    expect(result).toBe('An error occurred');
  });

  it('returns the default fallback for undefined', () => {
    const result = extractErrorMessage(undefined);

    expect(result).toBe('An error occurred');
  });

  it('returns the message from an Error subclass', () => {
    class CustomError extends Error {
      constructor(message: string) {
        super(message);
        this.name = 'CustomError';
      }
    }

    const result = extractErrorMessage(new CustomError('Custom error happened'));

    expect(result).toBe('Custom error happened');
  });

  it('returns a custom fallback for an object that is not an Error', () => {
    const result = extractErrorMessage({ code: 404 }, 'Not found');

    expect(result).toBe('Not found');
  });
});
