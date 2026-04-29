import { render, screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect } from 'vitest';
import { Sidebar } from '../Sidebar';

function renderSidebar() {
  return render(
    <MemoryRouter>
      <Sidebar />
    </MemoryRouter>,
  );
}

describe('Sidebar', () => {
  it('renders a navigation landmark', () => {
    renderSidebar();

    expect(screen.getByRole('navigation', { name: /main/i })).toBeInTheDocument();
  });

  it('renders the app name "Town Crier"', () => {
    renderSidebar();

    expect(screen.getByText('Town Crier')).toBeInTheDocument();
  });

  it('renders nav links for all sections', () => {
    renderSidebar();

    const nav = screen.getByRole('navigation', { name: /main/i });
    const expectedLinks = [
      'Dashboard',
      'Applications',
      'Saved',
      'Watch Zones',
      'Map',
      'Search',
      'Notifications',
      'Settings',
    ];

    for (const label of expectedLinks) {
      expect(within(nav).getByRole('link', { name: label })).toBeInTheDocument();
    }
  });

  it('renders exactly 8 nav links', () => {
    renderSidebar();

    const nav = screen.getByRole('navigation', { name: /main/i });
    const links = within(nav).getAllByRole('link');
    // 8 section links + 1 app name link = 9
    expect(links).toHaveLength(9);
  });

  it('does not render a Groups nav link', () => {
    renderSidebar();

    const nav = screen.getByRole('navigation', { name: /main/i });
    expect(within(nav).queryByRole('link', { name: 'Groups' })).not.toBeInTheDocument();
  });

  it('renders a Saved nav link pointing to /saved', () => {
    renderSidebar();

    const nav = screen.getByRole('navigation', { name: /main/i });
    const savedLink = within(nav).getByRole('link', { name: 'Saved' });
    expect(savedLink).toBeInTheDocument();
    expect(savedLink).toHaveAttribute('href', '/saved');
  });
});
