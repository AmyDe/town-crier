import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, beforeEach } from 'vitest';
import { ApplicationsPage } from '../ApplicationsPage';
import { SpyApplicationsBrowsePort } from './spies/spy-applications-browse-port';
import { SpyNotificationStateRepository } from './spies/spy-notification-state-repository';
import { cambridgeZone, oxfordZone } from './fixtures/zone.fixtures';
import {
  undecidedApplication,
  permittedApplication,
  rejectedApplication,
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

interface RenderInputs {
  zonesPort?: SpyZonesPort;
  browsePort?: SpyApplicationsBrowsePort;
  notificationStateRepository?: SpyNotificationStateRepository;
}

function renderPage({
  zonesPort,
  browsePort,
  notificationStateRepository,
}: RenderInputs = {}) {
  const zones = zonesPort ?? new SpyZonesPort();
  const browse = browsePort ?? new SpyApplicationsBrowsePort();
  const stateRepo =
    notificationStateRepository ?? new SpyNotificationStateRepository();
  return render(
    <MemoryRouter>
      <ApplicationsPage
        zonesPort={zones}
        browsePort={browse}
        notificationStateRepository={stateRepo}
      />
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
  it('renders a zone selector dropdown listing all real zones (no All option)', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone(), oxfordZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];

    renderPage({ zonesPort, browsePort });

    const selector = await screen.findByRole('combobox', { name: /zone/i });
    expect(selector).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'All' })).not.toBeInTheDocument();
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

  it('does not render a Saved toggle (moved to /saved)', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];

    renderPage({ zonesPort, browsePort });

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: /zone/i })).toBeInTheDocument();
    });
    expect(screen.queryByRole('switch', { name: /saved/i })).not.toBeInTheDocument();
  });
});

describe('ApplicationsPage — Unread chip', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('renders an Unread chip with the totalUnreadCount badge when unread > 0', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const stateRepo = new SpyNotificationStateRepository();
    stateRepo.getStateResult = {
      lastReadAt: '2026-01-01T00:00:00Z',
      version: 1,
      totalUnreadCount: 4,
    };

    renderPage({ zonesPort, browsePort, notificationStateRepository: stateRepo });

    const chip = await screen.findByRole('button', { name: /unread \(4\)/i });
    expect(chip).toBeInTheDocument();
    expect(chip).toHaveAttribute('aria-pressed', 'false');
  });

  it('hides the Unread chip when unreadCount is 0', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const stateRepo = new SpyNotificationStateRepository();
    stateRepo.getStateResult = {
      lastReadAt: '2026-01-01T00:00:00Z',
      version: 1,
      totalUnreadCount: 0,
    };

    renderPage({ zonesPort, browsePort, notificationStateRepository: stateRepo });

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: /zone/i })).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', { name: /unread/i })).not.toBeInTheDocument();
  });

  it('toggles the unread filter when the Unread chip is clicked', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [
      undecidedApplication({
        latestUnreadEvent: {
          type: 'NewApplication',
          decision: null,
          createdAt: '2026-04-01T00:00:00Z',
        },
      }),
      permittedApplication(), // null
    ];
    const stateRepo = new SpyNotificationStateRepository();
    stateRepo.getStateResult = {
      lastReadAt: '2026-01-01T00:00:00Z',
      version: 1,
      totalUnreadCount: 1,
    };
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort, notificationStateRepository: stateRepo });

    await waitFor(() =>
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument(),
    );
    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();

    await user.click(await screen.findByRole('button', { name: /unread \(1\)/i }));

    await waitFor(() => {
      expect(screen.queryByText('2026/0099/LBC')).not.toBeInTheDocument();
    });
    expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
  });
});

describe('ApplicationsPage — Mark all read', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('renders a Mark all read button when there are unread notifications', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const stateRepo = new SpyNotificationStateRepository();
    stateRepo.getStateResult = {
      lastReadAt: '2026-01-01T00:00:00Z',
      version: 1,
      totalUnreadCount: 3,
    };

    renderPage({ zonesPort, browsePort, notificationStateRepository: stateRepo });

    expect(
      await screen.findByRole('button', { name: /mark all read/i }),
    ).toBeInTheDocument();
  });

  it('hides the Mark all read button when nothing is unread', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const stateRepo = new SpyNotificationStateRepository();
    stateRepo.getStateResult = {
      lastReadAt: '2026-01-01T00:00:00Z',
      version: 1,
      totalUnreadCount: 0,
    };

    renderPage({ zonesPort, browsePort, notificationStateRepository: stateRepo });

    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: /zone/i })).toBeInTheDocument(),
    );
    expect(
      screen.queryByRole('button', { name: /mark all read/i }),
    ).not.toBeInTheDocument();
  });

  it('calls markAllRead on the repository when clicked', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];
    const stateRepo = new SpyNotificationStateRepository();
    stateRepo.getStateResult = {
      lastReadAt: '2026-01-01T00:00:00Z',
      version: 1,
      totalUnreadCount: 2,
    };
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort, notificationStateRepository: stateRepo });

    const button = await screen.findByRole('button', { name: /mark all read/i });
    await user.click(button);

    await waitFor(() => {
      expect(stateRepo.markAllReadCalls).toBe(1);
    });
  });
});

describe('ApplicationsPage — Sort menu', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('renders a sort selector defaulting to recent-activity', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];

    renderPage({ zonesPort, browsePort });

    const sort = await screen.findByRole('combobox', { name: /sort/i });
    expect(sort).toHaveValue('recent-activity');
    expect(
      screen.getByRole('option', { name: /recent activity/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /newest/i })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /oldest/i })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /^status$/i })).toBeInTheDocument();
  });

  it('changes sort order when a different option is selected', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [
      undecidedApplication({ startDate: '2026-01-01' }),
      permittedApplication({ startDate: '2026-03-01' }),
    ];
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort });

    await waitFor(() =>
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument(),
    );

    const sort = await screen.findByRole('combobox', { name: /sort/i });
    await user.selectOptions(sort, 'oldest');

    expect(sort).toHaveValue('oldest');
  });

  it('exposes a "Distance" option once a zone has been auto-selected', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];

    renderPage({ zonesPort, browsePort });

    // The auto-select effect in useApplications fires after zones load —
    // wait for the resulting Distance option (only emitted once a zone is
    // active) rather than racing the synchronous picker mount.
    await screen.findByRole('option', { name: /^distance$/i });
  });

  it('hides the "Distance" option in the multi-zone (no-zone) state', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = [];

    renderPage({ zonesPort, browsePort });

    await waitFor(() => expect(zonesPort.fetchZonesCalls).toBe(1));
    // No zones → no auto-selected zone → distance must not appear in any
    // sort surface. The picker itself only renders once a zone exists, so
    // simply assert the option is not in the document.
    expect(screen.queryByRole('option', { name: /^distance$/i })).toBeNull();
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
});
