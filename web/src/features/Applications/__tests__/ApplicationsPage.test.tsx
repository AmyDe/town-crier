import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { ApplicationsPage } from '../ApplicationsPage';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import { SpySavedApplicationRepository } from './spies/spy-saved-application-repository';
import { cambridgeZone, oxfordZone } from './fixtures/zone.fixtures';
import {
  undecidedApplication,
  permittedApplication,
  rejectedApplication,
} from '../../../components/ApplicationCard/__tests__/fixtures/planning-application-summary.fixtures';
import { savedUndecidedApplication, savedPermittedApplication } from './fixtures/saved-application.fixtures';
import { asApplicationUid, type WatchZoneSummary } from '../../../domain/types';

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

interface RenderInputs {
  zonesPort?: SpyZonesPort;
  browsePort?: SpyApplicationsBrowsePort;
  savedRepository?: SpySavedApplicationRepository;
}

function renderPage({ zonesPort, browsePort, savedRepository }: RenderInputs = {}) {
  const zones = zonesPort ?? new SpyZonesPort();
  const browse = browsePort ?? new SpyApplicationsBrowsePort();
  const saved = savedRepository ?? new SpySavedApplicationRepository();
  return render(
    <MemoryRouter>
      <ApplicationsPage zonesPort={zones} browsePort={browse} savedRepository={saved} />
    </MemoryRouter>,
  );
}

describe('ApplicationsPage — heading and zone bootstrap', () => {
  let zonesPort: SpyZonesPort;
  let browsePort: SpyApplicationsBrowsePort;

  beforeEach(() => {
    zonesPort = new SpyZonesPort();
    browsePort = new SpyApplicationsBrowsePort();
  });

  it('renders page heading', async () => {
    zonesPort.fetchZonesResult = [];
    renderPage({ zonesPort, browsePort });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Applications' })).toBeInTheDocument();
    });
  });

  it('shows empty state when user has no watch zones', async () => {
    zonesPort.fetchZonesResult = [];
    renderPage({ zonesPort, browsePort });

    await waitFor(() => {
      expect(
        screen.getByText('Set up a watch zone to start browsing applications.'),
      ).toBeInTheDocument();
    });
  });
});

describe('ApplicationsPage — filter bar', () => {
  it('renders a zone selector dropdown listing all zones plus an All option', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone(), oxfordZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];

    renderPage({ zonesPort, browsePort });

    const selector = await screen.findByRole('combobox', { name: /zone/i });
    expect(selector).toBeInTheDocument();
    // First zone is auto-selected; user can switch to others or to 'All'
    expect(screen.getByRole('option', { name: 'All' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Home - Cambridge' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Office - Oxford' })).toBeInTheDocument();
  });

  it('switches the active zone when a different option is chosen', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone(), oxfordZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication()];
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort });

    const selector = await screen.findByRole('combobox', { name: /zone/i });

    await user.selectOptions(selector, oxfordZone().id);

    await waitFor(() => {
      expect(browsePort.fetchByZoneCalls).toContain(oxfordZone().id);
    });
  });

  it('renders status filter chips', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];

    renderPage({ zonesPort, browsePort });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'All', pressed: true })).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: 'Pending' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Granted' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Refused' })).toBeInTheDocument();
  });

  it('renders a Saved toggle', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];

    renderPage({ zonesPort, browsePort });

    await waitFor(() => {
      const toggle = screen.getByRole('switch', { name: /saved/i });
      expect(toggle).toBeInTheDocument();
      expect(toggle).toHaveAttribute('aria-checked', 'false');
    });
  });
});

describe('ApplicationsPage — list rendering', () => {
  it("auto-selects the first zone and shows its applications", async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication(), permittedApplication()];

    renderPage({ zonesPort, browsePort });

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });
    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
  });

  it('filters list when a status chip is clicked', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [
      undecidedApplication(),
      permittedApplication(),
      rejectedApplication(),
    ];
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort });

    await waitFor(() => expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Granted' }));

    await waitFor(() => {
      expect(screen.queryByText('2026/0042/FUL')).not.toBeInTheDocument();
    });
    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
  });

  it('filters list to saved-only when the Saved toggle is on (real zone)', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [undecidedApplication(), permittedApplication()];
    const savedRepository = new SpySavedApplicationRepository();
    savedRepository.listSavedResult = [
      savedUndecidedApplication({ applicationUid: asApplicationUid('APP-001') }),
    ];
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort, savedRepository });

    await waitFor(() => expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument());

    await user.click(screen.getByRole('switch', { name: /saved/i }));

    await waitFor(() => {
      expect(screen.queryByText('2026/0099/LBC')).not.toBeInTheDocument();
    });
    expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
  });
});

describe('ApplicationsPage — All + Saved combinations', () => {
  it("shows hint when 'All' is selected with Saved off", async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort });

    const selector = await screen.findByRole('combobox', { name: /zone/i });
    await user.selectOptions(selector, '__all__');

    await waitFor(() => {
      expect(
        screen.getByText(
          /Pick a zone to see applications, or turn on Saved to see everything you've bookmarked\./,
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows all saved applications when 'All' is selected with Saved on", async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const savedRepository = new SpySavedApplicationRepository();
    savedRepository.listSavedResult = [savedUndecidedApplication(), savedPermittedApplication()];
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort, savedRepository });

    const selector = await screen.findByRole('combobox', { name: /zone/i });
    await user.selectOptions(selector, '__all__');
    await user.click(screen.getByRole('switch', { name: /saved/i }));

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });
    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
  });
});
