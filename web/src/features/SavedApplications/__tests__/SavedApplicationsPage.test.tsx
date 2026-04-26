import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import type { ReactNode } from 'react';
import { SavedApplicationsPage } from '../SavedApplicationsPage';
import { SpySavedApplicationRepository } from './spies/spy-saved-application-repository';
import {
  savedUndecidedApplication,
  savedPermittedApplication,
} from './fixtures/saved-application.fixtures';

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  };
}

describe('SavedApplicationsPage', () => {
  let spy: SpySavedApplicationRepository;

  beforeEach(() => {
    spy = new SpySavedApplicationRepository();
  });

  it('renders saved applications as cards', async () => {
    spy.listSavedResult = [savedUndecidedApplication(), savedPermittedApplication()];

    render(<SavedApplicationsPage repository={spy} />, {
      wrapper: createWrapper(),
    });

    expect(await screen.findByText('2026/0042/FUL')).toBeInTheDocument();
    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
  });

  it('renders the page heading', async () => {
    spy.listSavedResult = [savedUndecidedApplication()];

    render(<SavedApplicationsPage repository={spy} />, {
      wrapper: createWrapper(),
    });

    expect(await screen.findByRole('heading', { name: 'Saved Applications' })).toBeInTheDocument();
  });

  it('shows empty state when no saved applications', async () => {
    spy.listSavedResult = [];

    render(<SavedApplicationsPage repository={spy} />, {
      wrapper: createWrapper(),
    });

    expect(await screen.findByText('No saved applications')).toBeInTheDocument();
    expect(
      screen.getByText('Applications you save will appear here for quick access.'),
    ).toBeInTheDocument();
  });

  it('shows loading state initially', () => {
    render(<SavedApplicationsPage repository={spy} />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText('Loading saved applications...')).toBeInTheDocument();
  });

  it('shows error state when fetch fails', async () => {
    spy.listSavedError = new Error('Network unavailable');

    render(<SavedApplicationsPage repository={spy} />, {
      wrapper: createWrapper(),
    });

    expect(await screen.findByText('Network unavailable')).toBeInTheDocument();
  });

  it('renders a remove button for each card', async () => {
    spy.listSavedResult = [savedUndecidedApplication(), savedPermittedApplication()];

    render(<SavedApplicationsPage repository={spy} />, {
      wrapper: createWrapper(),
    });

    await screen.findByText('2026/0042/FUL');

    const removeButtons = screen.getAllByRole('button', { name: /remove/i });
    expect(removeButtons).toHaveLength(2);
  });

  it('calls remove when remove button is clicked and confirmed', async () => {
    const user = userEvent.setup();
    spy.listSavedResult = [savedUndecidedApplication()];

    render(<SavedApplicationsPage repository={spy} />, {
      wrapper: createWrapper(),
    });

    await screen.findByText('2026/0042/FUL');

    const removeButton = screen.getByRole('button', { name: /remove/i });
    await user.click(removeButton);

    // Confirm dialog should appear
    const dialog = screen.getByRole('dialog');
    expect(dialog).toBeInTheDocument();

    const confirmButton = within(dialog).getByRole('button', { name: 'Remove' });
    await user.click(confirmButton);

    expect(spy.removeCalls).toHaveLength(1);
    expect(spy.removeCalls[0]).toBe('APP-001');
  });

  it('does not call remove when cancel is clicked on confirm dialog', async () => {
    const user = userEvent.setup();
    spy.listSavedResult = [savedUndecidedApplication()];

    render(<SavedApplicationsPage repository={spy} />, {
      wrapper: createWrapper(),
    });

    await screen.findByText('2026/0042/FUL');

    const removeButton = screen.getByRole('button', { name: /remove/i });
    await user.click(removeButton);

    const dialog = screen.getByRole('dialog');
    const cancelButton = within(dialog).getByRole('button', { name: 'Cancel' });
    await user.click(cancelButton);

    expect(spy.removeCalls).toHaveLength(0);
  });
});
