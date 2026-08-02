import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect } from 'vitest';
import { ShapeModeToggle } from '../ShapeModeToggle';

describe('ShapeModeToggle', () => {
  it('renders Circle and Custom shape options as a radio group', () => {
    render(<ShapeModeToggle mode="circle" onSelect={() => {}} />);

    const group = screen.getByRole('radiogroup', { name: /shape/i });
    expect(group).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: /circle/i })).toBeInTheDocument();
    expect(screen.getByRole('radio', { name: /custom shape/i })).toBeInTheDocument();
  });

  it('marks the current mode as checked', () => {
    render(<ShapeModeToggle mode="custom" onSelect={() => {}} />);

    expect(screen.getByRole('radio', { name: /circle/i })).not.toBeChecked();
    expect(screen.getByRole('radio', { name: /custom shape/i })).toBeChecked();
  });

  it('calls onSelect with the clicked mode', async () => {
    const user = userEvent.setup();
    const selected: string[] = [];

    render(<ShapeModeToggle mode="circle" onSelect={(mode) => selected.push(mode)} />);

    await user.click(screen.getByRole('radio', { name: /custom shape/i }));

    expect(selected).toEqual(['custom']);
  });
});
