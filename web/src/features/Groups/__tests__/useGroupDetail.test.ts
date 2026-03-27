import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useGroupDetail } from '../useGroupDetail';
import { SpyGroupsRepository } from './spies/spy-groups-repository';
import { groupDetail, sampleInvitation } from './fixtures/group.fixtures';
import { asGroupId } from '../../../domain/types';

const GROUP_ID = asGroupId('group-001');

describe('useGroupDetail', () => {
  it('fetches group detail on mount', async () => {
    const spy = new SpyGroupsRepository();
    spy.getGroupResult = groupDetail();

    const { result } = renderHook(() => useGroupDetail(spy, GROUP_ID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.group?.name).toBe('Mill Road Residents');
    expect(result.current.group?.members).toHaveLength(2);
    expect(result.current.error).toBeNull();
    expect(spy.getGroupCalls).toContain(GROUP_ID);
  });

  it('sets error on failed fetch', async () => {
    const spy = new SpyGroupsRepository();
    spy.getGroupError = new Error('Not found');

    const { result } = renderHook(() => useGroupDetail(spy, GROUP_ID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Not found');
    expect(result.current.group).toBeNull();
  });

  it('sends an invitation and reloads the group', async () => {
    const spy = new SpyGroupsRepository();
    spy.getGroupResult = groupDetail();
    spy.inviteMemberResult = sampleInvitation();

    const { result } = renderHook(() => useGroupDetail(spy, GROUP_ID));

    await waitFor(() => {
      expect(result.current.group).not.toBeNull();
    });

    await act(async () => {
      await result.current.inviteMember('newuser@example.com');
    });

    expect(spy.inviteMemberCalls).toHaveLength(1);
    expect(spy.inviteMemberCalls[0]?.request.inviteeEmail).toBe('newuser@example.com');
    // Should reload group after invite
    expect(spy.getGroupCalls).toHaveLength(2);
  });

  it('removes a member and reloads the group', async () => {
    const spy = new SpyGroupsRepository();
    spy.getGroupResult = groupDetail();

    const { result } = renderHook(() => useGroupDetail(spy, GROUP_ID));

    await waitFor(() => {
      expect(result.current.group).not.toBeNull();
    });

    await act(async () => {
      await result.current.removeMember('user-member-1');
    });

    expect(spy.removeMemberCalls).toHaveLength(1);
    expect(spy.removeMemberCalls[0]?.memberUserId).toBe('user-member-1');
    expect(spy.getGroupCalls).toHaveLength(2);
  });

  it('deletes the group', async () => {
    const spy = new SpyGroupsRepository();
    spy.getGroupResult = groupDetail();

    const { result } = renderHook(() => useGroupDetail(spy, GROUP_ID));

    await waitFor(() => {
      expect(result.current.group).not.toBeNull();
    });

    await act(async () => {
      await result.current.deleteGroup();
    });

    expect(spy.deleteGroupCalls).toContain(GROUP_ID);
  });

  it('sets invite error when invitation fails', async () => {
    const spy = new SpyGroupsRepository();
    spy.getGroupResult = groupDetail();
    spy.inviteMemberError = new Error('Email already invited');

    const { result } = renderHook(() => useGroupDetail(spy, GROUP_ID));

    await waitFor(() => {
      expect(result.current.group).not.toBeNull();
    });

    await act(async () => {
      await result.current.inviteMember('duplicate@example.com');
    });

    expect(result.current.actionError).toBe('Email already invited');
  });
});
