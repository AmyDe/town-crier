import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, it, expect } from 'vitest';
import { LegalPage } from '../LegalPage';
import { SpyLegalDocumentPort } from './spies/spy-legal-document-port';
import { privacyPolicy, termsOfService } from './fixtures/legal-document.fixtures';

function renderLegalPage(type: string, spy?: SpyLegalDocumentPort) {
  const port = spy ?? new SpyLegalDocumentPort();
  return render(
    <MemoryRouter initialEntries={[`/legal/${type}`]}>
      <Routes>
        <Route path="/legal/:type" element={<LegalPage port={port} />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('LegalPage', () => {
  it('renders the document title as a heading', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentResult = privacyPolicy();
    renderLegalPage('privacy', spy);

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Privacy Policy' }),
    ).toBeInTheDocument();
  });

  it('renders each section heading and body', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentResult = privacyPolicy();
    renderLegalPage('privacy', spy);

    expect(
      await screen.findByRole('heading', { level: 2, name: 'What We Collect' }),
    ).toBeInTheDocument();
    expect(screen.getByText('We collect minimal data.')).toBeInTheDocument();

    expect(
      screen.getByRole('heading', { level: 2, name: 'Your Rights' }),
    ).toBeInTheDocument();
    expect(
      screen.getByText('You have the right to access your data.'),
    ).toBeInTheDocument();
  });

  it('renders last updated date', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentResult = privacyPolicy({ lastUpdated: '2026-03-16' });
    renderLegalPage('privacy', spy);

    expect(
      await screen.findByText(/last updated.*16 march 2026/i),
    ).toBeInTheDocument();
  });

  it('renders Terms of Service content', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentResult = termsOfService();
    renderLegalPage('terms', spy);

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Terms of Service' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { level: 2, name: 'Acceptance of Terms' }),
    ).toBeInTheDocument();
  });

  it('shows loading state while fetching', () => {
    const spy = new SpyLegalDocumentPort();
    // Default spy never resolves immediately in this test frame
    renderLegalPage('privacy', spy);

    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('shows error message when fetch fails', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentError = new Error('Network error');
    renderLegalPage('privacy', spy);

    expect(
      await screen.findByText(/network error/i),
    ).toBeInTheDocument();
  });

  it('fetches the correct document type from the URL param', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentResult = termsOfService();
    renderLegalPage('terms', spy);

    await screen.findByRole('heading', { level: 1, name: 'Terms of Service' });

    expect(spy.fetchDocumentCalls).toEqual(['terms']);
  });

  it('renders fallback heading for unknown legal type', async () => {
    const spy = new SpyLegalDocumentPort();
    spy.fetchDocumentResult = {
      documentType: 'unknown',
      title: 'Legal',
      lastUpdated: '2026-01-01',
      sections: [],
    };
    renderLegalPage('unknown', spy);

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Legal' }),
    ).toBeInTheDocument();
  });
});
