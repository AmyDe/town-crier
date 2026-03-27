import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { RadiusPicker } from '../RadiusPicker';

describe('RadiusPicker', () => {
  const radiusOptions = [
    { label: '1 km', metres: 1000 },
    { label: '2 km', metres: 2000 },
    { label: '5 km', metres: 5000 },
    { label: '10 km', metres: 10000 },
  ];

  it('renders all radius options', () => {
    render(<RadiusPicker selectedMetres={1000} onSelect={() => {}} />);

    for (const option of radiusOptions) {
      expect(screen.getByRole('radio', { name: option.label })).toBeInTheDocument();
    }
  });

  it('marks the selected radius as checked', () => {
    render(<RadiusPicker selectedMetres={5000} onSelect={() => {}} />);

    expect(screen.getByRole('radio', { name: '5 km' })).toBeChecked();
    expect(screen.getByRole('radio', { name: '1 km' })).not.toBeChecked();
  });

  it('calls onSelect with metres when a radius is clicked', async () => {
    const handleSelect = vi.fn();
    const user = userEvent.setup();

    render(<RadiusPicker selectedMetres={1000} onSelect={handleSelect} />);

    await user.click(screen.getByRole('radio', { name: '5 km' }));

    expect(handleSelect).toHaveBeenCalledWith(5000);
    expect(handleSelect).toHaveBeenCalledTimes(1);
  });

  it('renders with a group label for accessibility', () => {
    render(<RadiusPicker selectedMetres={1000} onSelect={() => {}} />);

    expect(screen.getByRole('radiogroup', { name: /radius/i })).toBeInTheDocument();
  });
});
