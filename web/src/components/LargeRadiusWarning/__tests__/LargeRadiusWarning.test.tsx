import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import {
  LargeRadiusWarning,
  LARGE_RADIUS_THRESHOLD_METRES,
} from '../LargeRadiusWarning';

describe('LargeRadiusWarning', () => {
  it('exposes a 2000m threshold matching the iOS sibling', () => {
    expect(LARGE_RADIUS_THRESHOLD_METRES).toBe(2000);
  });

  it('renders nothing below the threshold', () => {
    const { container } = render(<LargeRadiusWarning radiusMetres={1000} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('renders the warning copy at exactly the threshold', () => {
    render(<LargeRadiusWarning radiusMetres={2000} />);

    const alert = screen.getByRole('status');
    expect(alert).toHaveTextContent(/heads up/i);
    expect(alert).toHaveTextContent(/hundreds of notifications a day/i);
    expect(alert).toHaveTextContent(/100[–-]500\s?m/i);
    expect(alert).toHaveTextContent(/under 2\s?km/i);
  });

  it('renders the warning copy above the threshold', () => {
    render(<LargeRadiusWarning radiusMetres={5000} />);

    expect(screen.getByRole('status')).toHaveTextContent(
      /hundreds of notifications a day/i,
    );
  });
});
