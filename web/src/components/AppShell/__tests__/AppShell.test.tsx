import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { describe, it, expect } from 'vitest';
import { AppShell } from '../AppShell';

function renderAppShell(route = '/') {
  return render(
    <MemoryRouter initialEntries={[route]}>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<div>Page Content</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe('AppShell', () => {
  it('renders the sidebar navigation', () => {
    renderAppShell();

    expect(screen.getByRole('navigation', { name: /main/i })).toBeInTheDocument();
  });

  it('renders page content via Outlet', () => {
    renderAppShell();

    expect(screen.getByText('Page Content')).toBeInTheDocument();
  });

  it('renders a hamburger menu button', () => {
    renderAppShell();

    expect(screen.getByRole('button', { name: /menu/i })).toBeInTheDocument();
  });

  it('hamburger button toggles the mobile menu open', async () => {
    const user = userEvent.setup();
    renderAppShell();

    const menuButton = screen.getByRole('button', { name: /menu/i });
    expect(menuButton).toHaveAttribute('aria-expanded', 'false');

    await user.click(menuButton);
    expect(menuButton).toHaveAttribute('aria-expanded', 'true');
  });

  it('hamburger button toggles the mobile menu closed', async () => {
    const user = userEvent.setup();
    renderAppShell();

    const menuButton = screen.getByRole('button', { name: /menu/i });
    await user.click(menuButton);
    await user.click(menuButton);

    expect(menuButton).toHaveAttribute('aria-expanded', 'false');
  });

  it('mobile overlay is visible when menu is open', async () => {
    const user = userEvent.setup();
    renderAppShell();

    const menuButton = screen.getByRole('button', { name: /menu/i });
    await user.click(menuButton);

    expect(screen.getByTestId('mobile-overlay')).toBeInTheDocument();
  });

  it('clicking the overlay closes the mobile menu', async () => {
    const user = userEvent.setup();
    renderAppShell();

    const menuButton = screen.getByRole('button', { name: /menu/i });
    await user.click(menuButton);

    const overlay = screen.getByTestId('mobile-overlay');
    await user.click(overlay);

    expect(menuButton).toHaveAttribute('aria-expanded', 'false');
  });
});
