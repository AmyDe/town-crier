import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, beforeEach } from 'vitest';
import { ThemeToggle } from '../ThemeToggle';
import type { Theme } from '../../../hooks/useTheme';

describe('ThemeToggle', () => {
  let toggleCalls: number;
  const recordToggle = () => {
    toggleCalls++;
  };

  beforeEach(() => {
    toggleCalls = 0;
  });

  it('renders a button with aria-label "Switch to dark mode" when theme is light', () => {
    render(<ThemeToggle theme={'light' as Theme} toggleTheme={recordToggle} />);

    expect(
      screen.getByRole('button', { name: 'Switch to dark mode' }),
    ).toBeInTheDocument();
  });

  it('renders a button with aria-label "Switch to light mode" when theme is dark', () => {
    render(<ThemeToggle theme={'dark' as Theme} toggleTheme={recordToggle} />);

    expect(
      screen.getByRole('button', { name: 'Switch to light mode' }),
    ).toBeInTheDocument();
  });

  it('calls toggleTheme when clicked', async () => {
    const user = userEvent.setup();
    render(<ThemeToggle theme={'light' as Theme} toggleTheme={recordToggle} />);

    await user.click(screen.getByRole('button'));

    expect(toggleCalls).toBe(1);
  });

  it('shows sun icon when theme is dark', () => {
    const { container } = render(
      <ThemeToggle theme={'dark' as Theme} toggleTheme={recordToggle} />,
    );

    expect(container.querySelector('[data-icon="sun"]')).toBeInTheDocument();
  });

  it('shows moon icon when theme is light', () => {
    const { container } = render(
      <ThemeToggle theme={'light' as Theme} toggleTheme={recordToggle} />,
    );

    expect(container.querySelector('[data-icon="moon"]')).toBeInTheDocument();
  });
});
