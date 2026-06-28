import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect } from 'vitest';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { ApplicationCard } from '../ApplicationCard';
import {
  undecidedApplication,
  permittedApplication,
  conditionsApplication,
  rejectedApplication,
  longDescriptionApplication,
} from './fixtures/planning-application-summary.fixtures';

function renderCard(
  ...args: Parameters<typeof undecidedApplication>
) {
  const app = undecidedApplication(...args);
  return render(
    <MemoryRouter>
      <ApplicationCard application={app} />
    </MemoryRouter>,
  );
}

function LocationStateProbe() {
  const location = useLocation();
  const state = (location.state ?? {}) as { authority?: string; name?: string };
  return (
    <div>
      <span data-testid="state-authority">{state.authority ?? ''}</span>
      <span data-testid="state-name">{state.name ?? ''}</span>
    </div>
  );
}

describe('ApplicationCard', () => {
  it('renders the application reference name', () => {
    renderCard();

    expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
  });

  it('renders the address', () => {
    renderCard();

    expect(
      screen.getByText('12 Mill Road, Cambridge, CB1 2AD'),
    ).toBeInTheDocument();
  });

  it('renders the description', () => {
    renderCard();

    expect(
      screen.getByText(
        'Erection of two-storey rear extension with associated landscaping',
      ),
    ).toBeInTheDocument();
  });

  it('renders the application type', () => {
    renderCard();

    expect(screen.getByText('Full Planning')).toBeInTheDocument();
  });

  it('renders a status badge with the application state', () => {
    renderCard();

    const badge = screen.getByText('Undecided');
    expect(badge).toBeInTheDocument();
  });

  it('renders the area name', () => {
    renderCard();

    expect(
      screen.getByText('Cambridge City Council'),
    ).toBeInTheDocument();
  });

  it('renders the start date formatted for display', () => {
    renderCard();

    expect(screen.getByText('15 Jan 2026')).toBeInTheDocument();
  });

  it('links to the application detail page', () => {
    renderCard();

    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', '/applications/APP-001');
  });

  it('carries the authority and name in navigation state on the detail link', async () => {
    const user = userEvent.setup();
    const app = undecidedApplication();
    render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<ApplicationCard application={app} />} />
          <Route path="/applications/*" element={<LocationStateProbe />} />
        </Routes>
      </MemoryRouter>,
    );

    await user.click(screen.getByRole('link'));

    expect(screen.getByTestId('state-authority')).toHaveTextContent('42');
    expect(screen.getByTestId('state-name')).toHaveTextContent('2026/0042/FUL');
  });

  it('truncates long descriptions with an ellipsis', () => {
    const app = longDescriptionApplication();
    render(
      <MemoryRouter>
        <ApplicationCard application={app} />
      </MemoryRouter>,
    );

    const description = screen.getByTestId('application-description');
    const text = description.textContent ?? '';
    expect(text.length).toBeLessThanOrEqual(123);
    expect(text).toMatch(/\.\.\.$/);
  });

  it('renders "Granted" label for Permitted state', () => {
    const app = permittedApplication();
    render(
      <MemoryRouter>
        <ApplicationCard application={app} />
      </MemoryRouter>,
    );

    expect(screen.getByText('Granted')).toBeInTheDocument();
  });

  it('renders "Granted with conditions" label for Conditions state', () => {
    const app = conditionsApplication();
    render(
      <MemoryRouter>
        <ApplicationCard application={app} />
      </MemoryRouter>,
    );

    expect(screen.getByText('Granted with conditions')).toBeInTheDocument();
  });

  it('renders "Refused" label for Rejected state', () => {
    const app = rejectedApplication();
    render(
      <MemoryRouter>
        <ApplicationCard application={app} />
      </MemoryRouter>,
    );

    expect(screen.getByText('Refused')).toBeInTheDocument();
  });

  it('handles null start date gracefully', () => {
    renderCard({ startDate: null });

    expect(screen.queryByTestId('application-start-date')).not.toBeInTheDocument();
  });

  it('handles null description without crashing', () => {
    renderCard({ description: null as unknown as string });

    const description = screen.getByTestId('application-description');
    expect(description.textContent).toBe('');
  });
});

describe('ApplicationCard — leading unread dot', () => {
  it('renders a visible unread dot when latestUnreadEvent is non-null', () => {
    const app = undecidedApplication({
      latestUnreadEvent: {
        type: 'NewApplication',
        decision: null,
        createdAt: '2026-04-01T00:00:00Z',
      },
    });
    render(
      <MemoryRouter>
        <ApplicationCard application={app} />
      </MemoryRouter>,
    );

    const dot = screen.getByLabelText('Unread');
    expect(dot).toBeInTheDocument();
    expect(dot).toBeVisible();
  });

  it('keeps the unread dot in the DOM but hidden when latestUnreadEvent is null', () => {
    const app = permittedApplication({ latestUnreadEvent: null });
    render(
      <MemoryRouter>
        <ApplicationCard application={app} />
      </MemoryRouter>,
    );

    const dot = screen.getByTestId('application-unread-dot');
    expect(dot).toBeInTheDocument();
    expect(dot).not.toBeVisible();
  });

  it('marks the application card unread when latestUnreadEvent is non-null', () => {
    const app = undecidedApplication({
      latestUnreadEvent: {
        type: 'NewApplication',
        decision: null,
        createdAt: '2026-04-01T00:00:00Z',
      },
    });
    render(
      <MemoryRouter>
        <ApplicationCard application={app} />
      </MemoryRouter>,
    );

    const card = screen.getByTestId('application-card');
    expect(card).toHaveAttribute('data-unread', 'true');
  });

  it('does not apply mute styling to the status badge when read', () => {
    const app = permittedApplication({ latestUnreadEvent: null });
    render(
      <MemoryRouter>
        <ApplicationCard application={app} />
      </MemoryRouter>,
    );

    const badge = screen.getByTestId('application-status-badge');
    expect(badge).not.toHaveAttribute('data-unread');
  });
});
