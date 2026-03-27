import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useAcceptInvitation } from '../useAcceptInvitation';
import { SpyGroupsRepository } from './spies/spy-groups-repository';
import { asInvitationId } from '../../../domain/types';

const INVITATION_ID = asInvitationId('inv-001');

describe('useAcceptInvitation', () => {
  it('calls acceptInvitation on mount', async () => {
    const spy = new SpyGroupsRepository();

    renderHook(() => useAcceptInvitation(spy, INVITATION_ID));

    await waitFor(() => {
      expect(spy.acceptInvitationCalls).toContain(INVITATION_ID);
    });
  });

  it('sets success state after successful accept', async () => {
    const spy = new SpyGroupsRepository();

    const { result } = renderHook(() => useAcceptInvitation(spy, INVITATION_ID));

    await waitFor(() => {
      expect(result.current.isAccepted).toBe(true);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('sets error state on failed accept', async () => {
    const spy = new SpyGroupsRepository();
    spy.acceptInvitationError = new Error('Invitation expired');

    const { result } = renderHook(() => useAcceptInvitation(spy, INVITATION_ID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.isAccepted).toBe(false);
    expect(result.current.error).toBe('Invitation expired');
  });
});
