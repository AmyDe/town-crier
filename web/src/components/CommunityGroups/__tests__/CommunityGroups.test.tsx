import { render, screen, within } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { CommunityGroups } from '../CommunityGroups';

describe('CommunityGroups', () => {
  it('renders a section with id "community-groups" for anchor linking', () => {
    const { container } = render(<CommunityGroups />);

    const section = container.querySelector('section#community-groups');
    expect(section).toBeInTheDocument();
  });

  it('renders the heading "Stronger together"', () => {
    render(<CommunityGroups />);

    expect(
      screen.getByRole('heading', { name: /stronger together/i }),
    ).toBeInTheDocument();
  });

  it('renders a subheading about community action', () => {
    render(<CommunityGroups />);

    const subheading = screen.getByText(/planning decisions affect/i);
    expect(subheading).toBeInTheDocument();
    expect(subheading.className).toMatch(/subheading/);
  });

  it('renders three feature cards', () => {
    render(<CommunityGroups />);

    const list = screen.getByRole('list');
    const items = within(list).getAllByRole('listitem');
    expect(items).toHaveLength(3);
  });

  it('renders card titles: Create a group, Invite neighbours, Coordinate responses', () => {
    render(<CommunityGroups />);

    const items = screen.getAllByRole('listitem');

    expect(within(items[0]!).getByText(/create a group/i)).toBeInTheDocument();
    expect(within(items[1]!).getByText(/invite neighbours/i)).toBeInTheDocument();
    expect(within(items[2]!).getByText(/coordinate responses/i)).toBeInTheDocument();
  });

  it('renders card titles with amber styling class', () => {
    render(<CommunityGroups />);

    const items = screen.getAllByRole('listitem');

    for (const item of items) {
      const title = item.querySelector('h3');
      expect(title).toBeInTheDocument();
      expect(title?.className).toMatch(/featureTitle/);
    }
  });

  it('renders a description for each feature card', () => {
    render(<CommunityGroups />);

    const items = screen.getAllByRole('listitem');

    for (const item of items) {
      const paragraphs = within(item).getAllByText(/.+/);
      // Each item should have at least a title and a description
      expect(paragraphs.length).toBeGreaterThanOrEqual(2);
    }
  });

  it('marks feature icons as aria-hidden', () => {
    render(<CommunityGroups />);

    const items = screen.getAllByRole('listitem');

    for (const item of items) {
      const icon = item.querySelector('[aria-hidden="true"]');
      expect(icon).toBeInTheDocument();
    }
  });

  it('uses surface background on feature cards', () => {
    render(<CommunityGroups />);

    const items = screen.getAllByRole('listitem');

    for (const item of items) {
      expect(item.className).toMatch(/card/);
    }
  });
});
