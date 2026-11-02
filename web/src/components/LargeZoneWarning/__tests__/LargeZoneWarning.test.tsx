import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { LargeZoneWarning } from '../LargeZoneWarning';

describe('LargeZoneWarning', () => {
  it('renders nothing when the zone is not large', () => {
    const { container } = render(<LargeZoneWarning isLarge={false} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('renders the exact title and both paragraphs when the zone is large', () => {
    render(<LargeZoneWarning isLarge />);

    const status = screen.getByRole('status');
    expect(status).toHaveTextContent('Large zones can be slower to browse');
    expect(status).toHaveTextContent(
      'Your notifications work as normal, but the map and list can take a few seconds to load, especially in cities.',
    );
    expect(status).toHaveTextContent(
      'Just want to keep an eye on your street? Stick to 100-500 m in built-up areas, and under 2 km everywhere else.',
    );
  });

  it('no longer uses the old copy', () => {
    render(<LargeZoneWarning isLarge />);

    expect(screen.queryByText(/hundreds of notifications/i)).not.toBeInTheDocument();
  });
});
