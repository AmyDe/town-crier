import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect } from 'vitest';
import { SavedApplicationsPage } from '../SavedApplicationsPage';
import { SpySavedApplicationRepository } from '../../Applications/__tests__/spies/spy-saved-application-repository';
import {
  savedUndecidedApplication,
  savedPermittedApplication,
} from '../../Applications/__tests__/fixtures/saved-application.fixtures';
import { asApplicationUid } from '../../../domain/types';

interface RenderInputs {
  savedRepository?: SpySavedApplicationRepository;
}

function renderPage({ savedRepository }: RenderInputs = {}) {
  const repo = savedRepository ?? new SpySavedApplicationRepository();
  return render(
    <MemoryRouter>
      <SavedApplicationsPage savedRepository={repo} />
    </MemoryRouter>,
  );
}

describe('SavedApplicationsPage — heading', () => {
  it('renders the page heading', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [];

    renderPage({ savedRepository: repo });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Saved' })).toBeInTheDocument();
    });
  });
});

describe('SavedApplicationsPage — list rendering', () => {
  it('renders saved applications sorted newest first', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [
      savedUndecidedApplication({
        applicationUid: asApplicationUid('A'),
        savedAt: '2026-01-01T00:00:00Z',
      }),
      savedPermittedApplication({
        applicationUid: asApplicationUid('B'),
        savedAt: '2026-02-01T00:00:00Z',
      }),
    ];

    renderPage({ savedRepository: repo });

    await waitFor(() => {
      expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
    });
    expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
  });

  it('shows the empty-state copy when nothing is saved', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [];

    renderPage({ savedRepository: repo });

    await waitFor(() => {
      expect(
        screen.getByText(/Bookmark applications you want to track/i),
      ).toBeInTheDocument();
    });
  });

  it('shows the no-matches empty-state when a status filter excludes everything', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [
      savedUndecidedApplication({ applicationUid: asApplicationUid('A') }),
    ];
    const user = userEvent.setup();

    renderPage({ savedRepository: repo });

    await waitFor(() =>
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument(),
    );

    await user.click(screen.getByRole('button', { name: 'Granted' }));

    await waitFor(() => {
      expect(
        screen.getByText('No saved applications match this filter.'),
      ).toBeInTheDocument();
    });
  });

  it('renders an error message when the repository fails', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedError = new Error('Network unavailable');

    renderPage({ savedRepository: repo });

    await waitFor(() => {
      expect(screen.getByText('Network unavailable')).toBeInTheDocument();
    });
  });
});

describe('SavedApplicationsPage — status filter', () => {
  it('renders status filter chips', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [];

    renderPage({ savedRepository: repo });

    await waitFor(() => {
      expect(
        screen.getByRole('button', { name: 'All', pressed: true }),
      ).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: 'Pending' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Granted' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Refused' })).toBeInTheDocument();
  });

  it('filters the list when a status chip is clicked', async () => {
    const repo = new SpySavedApplicationRepository();
    repo.listSavedResult = [
      savedUndecidedApplication({
        applicationUid: asApplicationUid('A'),
        savedAt: '2026-01-01T00:00:00Z',
      }),
      savedPermittedApplication({
        applicationUid: asApplicationUid('B'),
        savedAt: '2026-02-01T00:00:00Z',
      }),
    ];
    const user = userEvent.setup();

    renderPage({ savedRepository: repo });

    await waitFor(() => {
      expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
    });
    expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Granted' }));

    await waitFor(() => {
      expect(screen.queryByText('2026/0042/FUL')).not.toBeInTheDocument();
    });
    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
  });
});
