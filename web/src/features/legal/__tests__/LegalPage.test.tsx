import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, it, expect } from 'vitest';
import { LegalPage } from '../LegalPage';

function renderLegalPage(type: string) {
  return render(
    <MemoryRouter initialEntries={[`/legal/${type}`]}>
      <Routes>
        <Route path="/legal/:type" element={<LegalPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('LegalPage', () => {
  it('renders Privacy Policy heading for /legal/privacy', () => {
    renderLegalPage('privacy');

    expect(
      screen.getByRole('heading', { level: 1, name: 'Privacy Policy' }),
    ).toBeInTheDocument();
  });

  it('renders Terms of Service heading for /legal/terms', () => {
    renderLegalPage('terms');

    expect(
      screen.getByRole('heading', { level: 1, name: 'Terms of Service' }),
    ).toBeInTheDocument();
  });

  it('renders fallback heading for unknown legal type', () => {
    renderLegalPage('unknown');

    expect(
      screen.getByRole('heading', { level: 1, name: 'Legal' }),
    ).toBeInTheDocument();
  });

  it('renders placeholder description text', () => {
    renderLegalPage('privacy');

    expect(screen.getByText('This page is coming soon.')).toBeInTheDocument();
  });
});
