import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { HowItWorks } from '../HowItWorks';

describe('HowItWorks', () => {
  it('renders a section with id "how-it-works" for anchor linking', () => {
    render(<HowItWorks />);

    const section = document.getElementById('how-it-works');
    expect(section).toBeInTheDocument();
    expect(section!.tagName).toBe('SECTION');
  });

  it('renders a heading', () => {
    render(<HowItWorks />);

    expect(
      screen.getByRole('heading', { name: /how it works/i }),
    ).toBeInTheDocument();
  });

  it('renders three steps in a semantic ordered list', () => {
    render(<HowItWorks />);

    const list = screen.getByRole('list');
    expect(list.tagName).toBe('OL');

    const items = within(list).getAllByRole('listitem');
    expect(items).toHaveLength(3);
  });

  it('renders step titles in order: postcode, watch zone, notified', () => {
    render(<HowItWorks />);

    const items = screen.getAllByRole('listitem');

    expect(within(items[0]!).getByText(/enter your postcode/i)).toBeInTheDocument();
    expect(within(items[1]!).getByText(/create a watch zone/i)).toBeInTheDocument();
    expect(within(items[2]!).getByText(/get notified/i)).toBeInTheDocument();
  });

  it('renders a description for each step', () => {
    render(<HowItWorks />);

    const items = screen.getAllByRole('listitem');

    for (const item of items) {
      const paragraphs = within(item).getAllByText(/.+/);
      // Each item should have at least a title and a description
      expect(paragraphs.length).toBeGreaterThanOrEqual(2);
    }
  });

  it('marks step icons as aria-hidden', () => {
    render(<HowItWorks />);

    const items = screen.getAllByRole('listitem');

    for (const item of items) {
      const icon = item.querySelector('[aria-hidden="true"]');
      expect(icon).toBeInTheDocument();
    }
  });
});
