import { renderHook, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useGroupCreate } from '../useGroupCreate';
import { SpyGroupsRepository } from './spies/spy-groups-repository';
import { groupDetail } from './fixtures/group.fixtures';
import { asAuthorityId } from '../../../domain/types';

describe('useGroupCreate', () => {
  it('starts at the postcode step', () => {
    const spy = new SpyGroupsRepository();

    const { result } = renderHook(() => useGroupCreate(spy));

    expect(result.current.step).toBe('postcode');
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('advances to radius step when location is set', () => {
    const spy = new SpyGroupsRepository();

    const { result } = renderHook(() => useGroupCreate(spy));

    act(() => {
      result.current.setLocation({ latitude: 52.2044, longitude: 0.1218 });
    });

    expect(result.current.step).toBe('radius');
  });

  it('advances to authority step when radius is set', () => {
    const spy = new SpyGroupsRepository();

    const { result } = renderHook(() => useGroupCreate(spy));

    act(() => {
      result.current.setLocation({ latitude: 52.2044, longitude: 0.1218 });
    });

    act(() => {
      result.current.setRadius(2000);
    });

    expect(result.current.step).toBe('authority');
  });

  it('advances to name step when authority is set', () => {
    const spy = new SpyGroupsRepository();

    const { result } = renderHook(() => useGroupCreate(spy));

    act(() => {
      result.current.setLocation({ latitude: 52.2044, longitude: 0.1218 });
    });

    act(() => {
      result.current.setRadius(2000);
    });

    act(() => {
      result.current.setAuthority({ id: asAuthorityId(101), name: 'Cambridge City Council', areaType: 'District' });
    });

    expect(result.current.step).toBe('name');
  });

  it('submits the group and returns the result', async () => {
    const spy = new SpyGroupsRepository();
    spy.createGroupResult = groupDetail();

    const { result } = renderHook(() => useGroupCreate(spy));

    act(() => {
      result.current.setLocation({ latitude: 52.2044, longitude: 0.1218 });
    });
    act(() => {
      result.current.setRadius(2000);
    });
    act(() => {
      result.current.setAuthority({ id: asAuthorityId(101), name: 'Cambridge City Council', areaType: 'District' });
    });

    let created: unknown;
    await act(async () => {
      created = await result.current.submit('Mill Road Residents');
    });

    expect(created).toBeDefined();
    expect(spy.createGroupCalls).toHaveLength(1);
    expect(spy.createGroupCalls[0]).toEqual({
      name: 'Mill Road Residents',
      latitude: 52.2044,
      longitude: 0.1218,
      radiusMetres: 2000,
      authorityId: 101,
    });
  });

  it('sets error on failed submission', async () => {
    const spy = new SpyGroupsRepository();
    spy.createGroupError = new Error('Group limit reached');

    const { result } = renderHook(() => useGroupCreate(spy));

    act(() => {
      result.current.setLocation({ latitude: 52.2044, longitude: 0.1218 });
    });
    act(() => {
      result.current.setRadius(2000);
    });
    act(() => {
      result.current.setAuthority({ id: asAuthorityId(101), name: 'Cambridge', areaType: 'District' });
    });

    await act(async () => {
      await result.current.submit('Test Group');
    });

    expect(result.current.error).toBe('Group limit reached');
  });
});
