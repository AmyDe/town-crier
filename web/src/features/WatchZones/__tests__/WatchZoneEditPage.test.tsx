import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { WatchZoneEditPage } from '../WatchZoneEditPage';
import { SpyWatchZoneRepository } from './spies/spy-watch-zone-repository';
import { aWatchZone, zonePreferences } from './fixtures/watch-zone.fixtures';

function renderWithRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('WatchZoneEditPage', () => {
  let spy: SpyWatchZoneRepository;

  beforeEach(() => {
    spy = new SpyWatchZoneRepository();
  });

  it('renders zone name and notification preferences', async () => {
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    expect(screen.getByText('Home')).toBeInTheDocument();

    // Wait for preferences to load
    const newAppsCheckbox = await screen.findByRole('checkbox', { name: /new applications/i });
    expect(newAppsCheckbox).toBeChecked();

    const statusCheckbox = screen.getByRole('checkbox', { name: /status changes/i });
    expect(statusCheckbox).toBeChecked();

    const decisionCheckbox = screen.getByRole('checkbox', { name: /decision updates/i });
    expect(decisionCheckbox).not.toBeChecked();
  });

  it('saves updated preferences on toggle', async () => {
    const user = userEvent.setup();
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    const decisionCheckbox = await screen.findByRole('checkbox', { name: /decision updates/i });

    // Update preferences result for the refetch
    spy.getPreferencesResult = zonePreferences({ decisionUpdates: true });

    await user.click(decisionCheckbox);

    expect(spy.updatePreferencesCalls).toHaveLength(1);
    expect(spy.updatePreferencesCalls[0]?.data.decisionUpdates).toBe(true);
  });

  it('shows loading state while preferences load', () => {
    spy.getPreferences = () => new Promise(() => {});

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('shows error when preferences fail to load', async () => {
    spy.getPreferencesError = new Error('Not found');

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    expect(await screen.findByText('Not found')).toBeInTheDocument();
  });

  it('shows zone details (radius)', async () => {
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    await screen.findByRole('checkbox', { name: /new applications/i });
    expect(screen.getByText('2 km radius')).toBeInTheDocument();
  });

  it('has a back link to the list', () => {
    spy.getPreferencesResult = zonePreferences();

    renderWithRouter(
      <WatchZoneEditPage repository={spy} zone={aWatchZone()} />,
    );

    const backLink = screen.getByRole('link', { name: /back to watch zones/i });
    expect(backLink).toHaveAttribute('href', '/watch-zones');
  });
});
