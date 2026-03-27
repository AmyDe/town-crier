import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useDesignations } from '../useDesignations';
import { SpyDesignationRepository } from './spies/spy-designation-repository';
import { conservationAreaDesignation } from './fixtures/designation-context.fixtures';

describe('useDesignations', () => {
  it('fetches designations when coordinates are provided', async () => {
    const spy = new SpyDesignationRepository();
    const expected = conservationAreaDesignation();
    spy.fetchDesignationsResult = expected;

    const { result } = renderHook(() =>
      useDesignations(spy, 52.2053, 0.1218),
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(spy.fetchDesignationsCalls).toEqual([
      { latitude: 52.2053, longitude: 0.1218 },
    ]);
    expect(result.current.designations).toEqual(expected);
    expect(result.current.error).toBeNull();
  });

  it('does not fetch when latitude is null', () => {
    const spy = new SpyDesignationRepository();

    const { result } = renderHook(() =>
      useDesignations(spy, null, 0.1218),
    );

    expect(spy.fetchDesignationsCalls).toHaveLength(0);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.designations).toBeNull();
  });

  it('does not fetch when longitude is null', () => {
    const spy = new SpyDesignationRepository();

    const { result } = renderHook(() =>
      useDesignations(spy, 52.2053, null),
    );

    expect(spy.fetchDesignationsCalls).toHaveLength(0);
    expect(result.current.isLoading).toBe(false);
    expect(result.current.designations).toBeNull();
  });

  it('sets error when fetch fails', async () => {
    const spy = new SpyDesignationRepository();
    spy.fetchDesignationsError = new Error('Designation lookup failed');

    const { result } = renderHook(() =>
      useDesignations(spy, 52.2053, 0.1218),
    );

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Designation lookup failed');
    expect(result.current.designations).toBeNull();
  });
});
