import { renderHook, waitFor } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useLegalDocument } from '../useLegalDocument';
import { SpyLegalDocumentPort } from './spies/spy-legal-document-port';
import { privacyPolicy, termsOfService } from './fixtures/legal-document.fixtures';

describe('useLegalDocument', () => {
  it('fetches document on mount and exposes content', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentResult = privacyPolicy();

    const { result } = renderHook(() => useLegalDocument(spy, 'privacy'));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.document).toEqual(privacyPolicy());
    expect(result.current.error).toBeNull();
    expect(spy.fetchDocumentCalls).toEqual(['privacy']);
  });

  it('passes the correct document type to the port', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentResult = termsOfService();

    const { result } = renderHook(() => useLegalDocument(spy, 'terms'));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.document).toEqual(termsOfService());
    expect(spy.fetchDocumentCalls).toEqual(['terms']);
  });

  it('exposes loading state initially', () => {
    const spy = new SpyLegalDocumentPort();

    const { result } = renderHook(() => useLegalDocument(spy, 'privacy'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.document).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('exposes error when fetch fails', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentError = new Error('Network error');

    const { result } = renderHook(() => useLegalDocument(spy, 'privacy'));

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.document).toBeNull();
  });
});
