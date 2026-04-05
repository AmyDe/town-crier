import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { ApplicationCard } from '../ApplicationCard';
import {
  undecidedApplication,
  approvedApplication,
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

  it('renders different status states correctly', () => {
    const app = approvedApplication();
    render(
      <MemoryRouter>
        <ApplicationCard application={app} />
      </MemoryRouter>,
    );

    expect(screen.getByText('Approved')).toBeInTheDocument();
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
