import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { ApplicationsPage } from '../ApplicationsPage';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import { cambridgeZone, oxfordZone } from './fixtures/zone.fixtures';
import {
  undecidedApplication,
  permittedApplication,
} from '../../../components/ApplicationCard/__tests__/fixtures/planning-application-summary.fixtures';
import type { WatchZoneSummary } from '../../../domain/types';

class SpyZonesPort {
  fetchZonesCalls = 0;
  fetchZonesResult: readonly WatchZoneSummary[] = [];
  fetchZonesError: Error | null = null;

  async fetchZones(): Promise<readonly WatchZoneSummary[]> {
    this.fetchZonesCalls++;
    if (this.fetchZonesError) {
      throw this.fetchZonesError;
    }
    return this.fetchZonesResult;
  }
}

function renderPage(
  zonesPort: SpyZonesPort,
  browsePort: SpyApplicationsBrowsePort,
) {
  return render(
    <MemoryRouter>
      <ApplicationsPage zonesPort={zonesPort} browsePort={browsePort} />
    </MemoryRouter>,
  );
}

describe('ApplicationsPage', () => {
  let zonesPort: SpyZonesPort;
  let browsePort: SpyApplicationsBrowsePort;

  beforeEach(() => {
    zonesPort = new SpyZonesPort();
    browsePort = new SpyApplicationsBrowsePort();
  });

  it('renders page heading', async () => {
    zonesPort.fetchZonesResult = [];
    renderPage(zonesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Applications' })).toBeInTheDocument();
    });
  });

  it('shows loading state while fetching zones', () => {
    renderPage(zonesPort, browsePort);

    expect(screen.getByText('Loading zones...')).toBeInTheDocument();
  });

  it('shows empty state when user has no watch zones', async () => {
    zonesPort.fetchZonesResult = [];
    renderPage(zonesPort, browsePort);

    await waitFor(() => {
      expect(
        screen.getByText('Set up a watch zone to start browsing applications.'),
      ).toBeInTheDocument();
    });
  });

  it('shows zone cards when user has watch zones', async () => {
    zonesPort.fetchZonesResult = [cambridgeZone(), oxfordZone()];
    renderPage(zonesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Home - Cambridge')).toBeInTheDocument();
    });
    expect(screen.getByText('Office - Oxford')).toBeInTheDocument();
  });

  it('shows applications when zone card is clicked', async () => {
    zonesPort.fetchZonesResult = [cambridgeZone()];
    browsePort.fetchByZoneResult = [undecidedApplication(), permittedApplication()];
    const user = userEvent.setup();

    renderPage(zonesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Home - Cambridge')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Home - Cambridge'));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
    expect(browsePort.fetchByZoneCalls).toEqual([cambridgeZone().id]);
  });

  it('shows breadcrumb when viewing applications', async () => {
    zonesPort.fetchZonesResult = [cambridgeZone()];
    browsePort.fetchByZoneResult = [undecidedApplication()];
    const user = userEvent.setup();

    renderPage(zonesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Home - Cambridge')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Home - Cambridge'));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: 'Watch Zones' })).toBeInTheDocument();
  });

  it('returns to zone list when breadcrumb is clicked', async () => {
    zonesPort.fetchZonesResult = [cambridgeZone(), oxfordZone()];
    browsePort.fetchByZoneResult = [undecidedApplication()];
    const user = userEvent.setup();

    renderPage(zonesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Home - Cambridge')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Home - Cambridge'));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Watch Zones' }));

    await waitFor(() => {
      expect(screen.getByText('Office - Oxford')).toBeInTheDocument();
    });
    expect(screen.getByText('Home - Cambridge')).toBeInTheDocument();
  });

  it('shows empty state when zone has no applications', async () => {
    zonesPort.fetchZonesResult = [cambridgeZone()];
    browsePort.fetchByZoneResult = [];
    const user = userEvent.setup();

    renderPage(zonesPort, browsePort);

    await waitFor(() => {
      expect(screen.getByText('Home - Cambridge')).toBeInTheDocument();
    });

    await user.click(screen.getByText('Home - Cambridge'));

    await waitFor(() => {
      expect(
        screen.getByText('No applications found in this zone.'),
      ).toBeInTheDocument();
    });
  });
});
