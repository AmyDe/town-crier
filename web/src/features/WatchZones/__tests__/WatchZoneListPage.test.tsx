import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { WatchZoneListPage } from '../WatchZoneListPage';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { aWatchZone, aSecondWatchZone } from './fixtures/watch-zone.fixtures';

function renderWithRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('WatchZoneListPage', () => {
  let spy: SpyWatchZoneRepository;

  beforeEach(() => {
    spy = new SpyWatchZoneRepository();
  });

  it('renders zone cards with name and radius', async () => {
    spy.listResult = [aWatchZone(), aSecondWatchZone()];

    renderWithRouter(<WatchZoneListPage repository={spy} />);

    expect(await screen.findByText('Home')).toBeInTheDocument();
    expect(screen.getByText('2 km')).toBeInTheDocument();
    expect(screen.getByText('Office')).toBeInTheDocument();
    expect(screen.getByText('5 km')).toBeInTheDocument();
  });

  it('renders a "Create Watch Zone" link', async () => {
    spy.listResult = [aWatchZone()];

    renderWithRouter(<WatchZoneListPage repository={spy} />);

    await screen.findByText('Home');

    const link = screen.getByRole('link', { name: /create watch zone/i });
    expect(link).toHaveAttribute('href', '/watch-zones/new');
  });

  it('renders empty state when no zones exist', async () => {
    spy.listResult = [];

    renderWithRouter(<WatchZoneListPage repository={spy} />);

    expect(await screen.findByText(/no watch zones yet/i)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /create your first watch zone/i })).toBeInTheDocument();
  });

  it('shows loading state', () => {
    spy.listResult = [];
    // Don't resolve the promise — stays loading
    spy.list = () => new Promise(() => {});

    renderWithRouter(<WatchZoneListPage repository={spy} />);

    expect(screen.getByRole('heading', { name: /watch zones/i })).toBeInTheDocument();
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('shows error message on failure', async () => {
    spy.listError = new Error('Network unavailable');

    renderWithRouter(<WatchZoneListPage repository={spy} />);

    expect(await screen.findByText('Network unavailable')).toBeInTheDocument();
  });

  it('deletes zone after confirmation', async () => {
    const user = userEvent.setup();
    spy.listResult = [aWatchZone(), aSecondWatchZone()];

    renderWithRouter(<WatchZoneListPage repository={spy} />);

    await screen.findByText('Home');

    const deleteButtons = screen.getAllByRole('button', { name: /delete/i });
    await user.click(deleteButtons[0]!);

    // Confirm dialog appears
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText(/are you sure/i)).toBeInTheDocument();

    // After confirming, delete is called
    spy.listResult = [aSecondWatchZone()];
    await user.click(screen.getByRole('button', { name: /confirm/i }));

    expect(spy.deleteCalls).toEqual(['zone-1']);
  });

  it('renders edit links for each zone', async () => {
    spy.listResult = [aWatchZone()];

    renderWithRouter(<WatchZoneListPage repository={spy} />);

    await screen.findByText('Home');

    const editLink = screen.getByRole('link', { name: /edit/i });
    expect(editLink).toHaveAttribute('href', '/watch-zones/zone-1');
  });
});
