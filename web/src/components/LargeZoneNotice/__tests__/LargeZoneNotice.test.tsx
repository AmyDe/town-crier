import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { LargeZoneNotice, LargeZoneLoadingMessage } from '../LargeZoneNotice';

describe('LargeZoneNotice', () => {
  it('renders nothing when the zone is not large', () => {
    const { container } = render(<LargeZoneNotice isLarge={false} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('renders the exact one-line notice for a large zone', () => {
    render(<LargeZoneNotice isLarge />);

    expect(
      screen.getByText(
        'Large zone: built for notifications. This can take up to a minute to load the first time.',
      ),
    ).toBeInTheDocument();
  });

  it('has no dismiss control', () => {
    render(<LargeZoneNotice isLarge />);

    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});

describe('LargeZoneLoadingMessage', () => {
  it('renders nothing when hidden', () => {
    const { container } = render(<LargeZoneLoadingMessage isVisible={false} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('renders the exact loading message as a status', () => {
    render(<LargeZoneLoadingMessage isVisible />);

    expect(screen.getByRole('status')).toHaveTextContent(
      'Loading a large zone. This can take up to a minute the first time.',
    );
  });
});
