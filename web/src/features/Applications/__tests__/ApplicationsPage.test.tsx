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
import { asApplicationUid } from '../../../domain/types';
import type { PlanningApplicationSummary, WatchZoneSummary } from '../../../domain/types';

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

/** Convenience: a single-page result (no further pages). */
function onePage(rows: readonly PlanningApplicationSummary[]) {
  return { rows, nextCursor: null };
}

describe('ApplicationsPage — heading and zone bootstrap', () => {
  it('renders page heading', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [];
    renderPage({ zonesPort });

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Applications' })).toBeInTheDocument();
    });
  });

  it('shows empty state when user has no watch zones', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [];
    renderPage({ zonesPort });

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

    renderPage({ zonesPort, browsePort });

    const selector = await screen.findByRole('combobox', { name: /zone/i });
    expect(selector).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'All' })).not.toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Home - Cambridge' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'Office - Oxford' })).toBeInTheDocument();
  });

  it('switches the active zone (server-side fetch) when a different option is chosen', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone(), oxfordZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = onePage([undecidedApplication()]);
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort });

    const selector = await screen.findByRole('combobox', { name: /zone/i });
    await user.selectOptions(selector, oxfordZone().id);

    await waitFor(() => {
      expect(browsePort.fetchByZoneCalls.some((c) => c.zoneId === oxfordZone().id)).toBe(true);
    });
  });

  it('renders status filter chips', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();

    renderPage({ zonesPort, browsePort });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'All', pressed: true })).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: 'Pending' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Granted' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Refused' })).toBeInTheDocument();
  });
});

describe('ApplicationsPage — Unread chip', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("renders an Unread chip whose badge equals the zone's whole unread total (not loaded rows)", async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    // Two loaded rows carry an unread event, but the whole zone has five —
    // the chip must reflect the zone total, not the rows on screen.
    browsePort.fetchByZoneResult = onePage([
      undecidedApplication({
        latestUnreadEvent: { type: 'NewApplication', decision: null, createdAt: '2026-04-01T00:00:00Z' },
      }),
      permittedApplication(),
      rejectedApplication({
        latestUnreadEvent: { type: 'DecisionUpdate', decision: 'Rejected', createdAt: '2026-04-15T00:00:00Z' },
      }),
    ]);
    browsePort.unreadTotal = 5;

    renderPage({ zonesPort, browsePort });

    const chip = await screen.findByRole('button', { name: /unread \(5\)/i });
    expect(chip).toBeInTheDocument();
    expect(chip).toHaveAttribute('aria-pressed', 'false');
  });

  it('hides the Unread chip when the zone has no unread applications', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = onePage([undecidedApplication(), permittedApplication()]);

    renderPage({ zonesPort, browsePort });

    await waitFor(() => {
      expect(screen.getByRole('combobox', { name: /zone/i })).toBeInTheDocument();
    });
    expect(screen.queryByRole('button', { name: /unread/i })).not.toBeInTheDocument();
  });

  it('refetches with unread-only (server-side) when the Unread chip is clicked', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    const unread = undecidedApplication({
      latestUnreadEvent: { type: 'NewApplication', decision: null, createdAt: '2026-04-01T00:00:00Z' },
    });
    const read = permittedApplication();
    browsePort.fetchByZoneResponder = (q) => onePage(q.unread ? [unread] : [unread, read]);
    browsePort.unreadTotal = 1;
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort });

    await waitFor(() => expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument());
    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();

    await user.click(await screen.findByRole('button', { name: /unread \(1\)/i }));

    await waitFor(() => {
      expect(screen.queryByText('2026/0099/LBC')).not.toBeInTheDocument();
    });
    expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    expect(browsePort.fetchByZoneCalls.at(-1)!.unread).toBe(true);
  });
});

describe('ApplicationsPage — Mark all read', () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it('renders a Mark all read button when at least one loaded application is unread', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = onePage([
      undecidedApplication({
        latestUnreadEvent: { type: 'NewApplication', decision: null, createdAt: '2026-04-01T00:00:00Z' },
      }),
    ]);
    browsePort.unreadTotal = 1;

    renderPage({ zonesPort, browsePort });

    expect(await screen.findByRole('button', { name: /mark all read/i })).toBeInTheDocument();
  });

  it('calls markAllRead on the repository when clicked', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = onePage([
      undecidedApplication({
        latestUnreadEvent: { type: 'NewApplication', decision: null, createdAt: '2026-04-01T00:00:00Z' },
      }),
    ]);
    browsePort.unreadTotal = 1;
    const stateRepo = new SpyNotificationStateRepository();
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

    renderPage({ zonesPort, browsePort });

    const sort = await screen.findByRole('combobox', { name: /sort/i });
    expect(sort).toHaveValue('recent-activity');
    expect(screen.getByRole('option', { name: /recent activity/i })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /newest/i })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /oldest/i })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: /^status$/i })).toBeInTheDocument();
  });

  it('refetches with the chosen sort server-side', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = onePage([undecidedApplication()]);
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort });

    await waitFor(() => expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument());

    const sort = await screen.findByRole('combobox', { name: /sort/i });
    await user.selectOptions(sort, 'oldest');

    expect(sort).toHaveValue('oldest');
    await waitFor(() => expect(browsePort.fetchByZoneCalls.at(-1)!.sort).toBe('oldest'));
    expect(browsePort.fetchByZoneCalls.at(-1)!.cursor).toBeNull();
  });

  it('exposes a "Distance" option once a zone has been auto-selected', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();

    renderPage({ zonesPort, browsePort });

    await screen.findByRole('option', { name: /^distance$/i });
  });

  it('hides the "Distance" option in the multi-zone (no-zone) state', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [];
    const browsePort = new SpyApplicationsBrowsePort();

    renderPage({ zonesPort, browsePort });

    await waitFor(() => expect(zonesPort.fetchZonesCalls).toBe(1));
    expect(screen.queryByRole('option', { name: /^distance$/i })).toBeNull();
  });
});

describe('ApplicationsPage — list rendering', () => {
  it('auto-selects the first zone and shows its applications', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = onePage([undecidedApplication(), permittedApplication()]);

    renderPage({ zonesPort, browsePort });

    await waitFor(() => {
      expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument();
    });
    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
  });

  it('refetches with the status filter (server-side) when a status chip is clicked', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResponder = (q) => {
      const all = [undecidedApplication(), permittedApplication(), rejectedApplication()];
      return onePage(q.status === null ? all : all.filter((a) => a.appState === q.status));
    };
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort });

    await waitFor(() => expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Granted' }));

    await waitFor(() => {
      expect(screen.queryByText('2026/0042/FUL')).not.toBeInTheDocument();
    });
    expect(screen.getByText('2026/0099/LBC')).toBeInTheDocument();
    expect(browsePort.fetchByZoneCalls.at(-1)!.status).toBe('Permitted');
  });
});

describe('ApplicationsPage — Load more (keyset pagination)', () => {
  it('appends the next page when Load more is clicked, then hides the button on the last page', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    const first = undecidedApplication({ name: 'FIRST/0001' });
    const second = { ...permittedApplication(), uid: asApplicationUid('SECOND-1'), name: 'SECOND/0002' };
    browsePort.fetchByZoneResponder = (q) =>
      q.cursor === null ? { rows: [first], nextCursor: 'c1' } : { rows: [second], nextCursor: null };
    const user = userEvent.setup();

    renderPage({ zonesPort, browsePort });

    await waitFor(() => expect(screen.getByText('FIRST/0001')).toBeInTheDocument());
    const loadMore = screen.getByRole('button', { name: /load more/i });

    await user.click(loadMore);

    await waitFor(() => expect(screen.getByText('SECOND/0002')).toBeInTheDocument());
    // First page row is still present (appended, not replaced).
    expect(screen.getByText('FIRST/0001')).toBeInTheDocument();
    // Last page reached — the button is gone.
    expect(screen.queryByRole('button', { name: /load more/i })).not.toBeInTheDocument();
  });

  it('does not render Load more when the first page is already the last', async () => {
    const zonesPort = new SpyZonesPort();
    zonesPort.fetchZonesResult = [cambridgeZone()];
    const browsePort = new SpyApplicationsBrowsePort();
    browsePort.fetchByZoneResult = onePage([undecidedApplication()]);

    renderPage({ zonesPort, browsePort });

    await waitFor(() => expect(screen.getByText('2026/0042/FUL')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /load more/i })).not.toBeInTheDocument();
  });
});
