import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect } from 'vitest';
import { NotificationsPage } from '../NotificationsPage';
import { SpyNotificationRepository } from './spies/spy-notification-repository';
import { aNotification, aSecondNotification, notificationsPage } from './fixtures/notification.fixtures';

function renderPage(spy: SpyNotificationRepository) {
  return render(
    <MemoryRouter>
      <NotificationsPage repository={spy} />
    </MemoryRouter>,
  );
}

describe('NotificationsPage', () => {
  it('renders notification items with application name and timestamp', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage();

    renderPage(spy);

    await waitFor(() => {
      expect(screen.getByText('2026/0042')).toBeInTheDocument();
    });

    expect(screen.getByText('2026/0099')).toBeInTheDocument();
    // Timestamps are displayed
    expect(screen.getByText(/15 Mar 2026/)).toBeInTheDocument();
    expect(screen.getByText(/14 Mar 2026/)).toBeInTheDocument();
  });

  it('shows the page heading', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage();

    renderPage(spy);

    expect(screen.getByRole('heading', { name: 'Notifications' })).toBeInTheDocument();
  });

  it('shows empty state when there are no notifications', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage([], 0, 1);

    renderPage(spy);

    await waitFor(() => {
      expect(screen.getByText(/no notifications/i)).toBeInTheDocument();
    });
  });

  it('shows error state on failure', async () => {
    const spy = new SpyNotificationRepository();
    spy.listError = new Error('Something went wrong');

    renderPage(spy);

    await waitFor(() => {
      expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    });
  });

  it('displays pagination when more than one page', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage([aNotification()], 40, 1);

    renderPage(spy);

    await waitFor(() => {
      expect(screen.getByText('2026/0042')).toBeInTheDocument();
    });

    expect(screen.getByText('Page 1 of 2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /next/i })).toBeEnabled();
    expect(screen.getByRole('button', { name: /previous/i })).toBeDisabled();
  });

  it('navigates to next page when Next is clicked', async () => {
    const user = userEvent.setup();
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage([aNotification()], 40, 1);

    renderPage(spy);

    await waitFor(() => {
      expect(screen.getByText('2026/0042')).toBeInTheDocument();
    });

    spy.listResult = notificationsPage([aSecondNotification()], 40, 2);

    await user.click(screen.getByRole('button', { name: /next/i }));

    await waitFor(() => {
      expect(screen.getByText('2026/0099')).toBeInTheDocument();
    });

    expect(screen.getByText('Page 2 of 2')).toBeInTheDocument();
  });

  it('shows notification address and type', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage([aNotification()], 1, 1);

    renderPage(spy);

    await waitFor(() => {
      expect(screen.getByText('12 Mill Road, Cambridge, CB1 2AD')).toBeInTheDocument();
    });

    expect(screen.getByText('Full')).toBeInTheDocument();
  });

  it('does not show pagination when there is only one page', async () => {
    const spy = new SpyNotificationRepository();
    spy.listResult = notificationsPage([aNotification()], 1, 1);

    renderPage(spy);

    await waitFor(() => {
      expect(screen.getByText('2026/0042')).toBeInTheDocument();
    });

    expect(screen.queryByText(/Page \d/)).not.toBeInTheDocument();
  });
});
