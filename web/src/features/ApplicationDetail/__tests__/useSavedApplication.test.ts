import { renderHook, waitFor, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useSavedApplication } from '../useSavedApplication';
import { SpySavedApplicationRepository } from './spies/spy-saved-application-repository';
import { fullApplication } from './fixtures/planning-application.fixtures';
import { asApplicationUid } from '../../../domain/types';

const APP_UID = asApplicationUid('APP-001');
const APP = fullApplication({ uid: APP_UID });

describe('useSavedApplication', () => {
  it('detects the application is saved when it appears in the saved list', async () => {
    const spy = new SpySavedApplicationRepository();
    spy.listSavedResult = [{ applicationUid: APP_UID, savedAt: '2026-03-01' }];

    const { result } = renderHook(() => useSavedApplication(spy, APP_UID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.isSaved).toBe(true);
  });

  it('detects the application is not saved when absent from the list', async () => {
    const spy = new SpySavedApplicationRepository();
    spy.listSavedResult = [];

    const { result } = renderHook(() => useSavedApplication(spy, APP_UID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.isSaved).toBe(false);
  });

  it('saves the full application when toggled from unsaved', async () => {
    const spy = new SpySavedApplicationRepository();
    spy.listSavedResult = [];

    const { result } = renderHook(() => useSavedApplication(spy, APP_UID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.toggleSave(APP);
    });

    expect(spy.saveCalls).toEqual([APP]);
    expect(result.current.isSaved).toBe(true);
  });

  it('removes the application when toggled from saved', async () => {
    const spy = new SpySavedApplicationRepository();
    spy.listSavedResult = [{ applicationUid: APP_UID, savedAt: '2026-03-01' }];

    const { result } = renderHook(() => useSavedApplication(spy, APP_UID));

    await waitFor(() => {
      expect(result.current.isSaved).toBe(true);
    });

    await act(async () => {
      await result.current.toggleSave(APP);
    });

    expect(spy.removeCalls).toEqual([APP_UID]);
    expect(result.current.isSaved).toBe(false);
  });

  it('reverts on save error', async () => {
    const spy = new SpySavedApplicationRepository();
    spy.listSavedResult = [];
    spy.saveError = new Error('Save failed');

    const { result } = renderHook(() => useSavedApplication(spy, APP_UID));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    await act(async () => {
      await result.current.toggleSave(APP);
    });

    expect(result.current.isSaved).toBe(false);
    expect(result.current.error).toBe('Save failed');
  });
});
