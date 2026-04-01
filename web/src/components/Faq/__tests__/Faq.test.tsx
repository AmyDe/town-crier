import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { Faq } from '../Faq';

describe('Faq', () => {
  it('renders a section with id="faq"', () => {
    const { container } = render(<Faq />);

    const section = container.querySelector('section#faq');
    expect(section).toBeInTheDocument();
  });

  it('renders a section heading', () => {
    render(<Faq />);

    expect(
      screen.getByRole('heading', { name: /frequently asked questions/i }),
    ).toBeInTheDocument();
  });

  it('renders five details/summary elements', () => {
    const { container } = render(<Faq />);

    const details = container.querySelectorAll('details');
    expect(details).toHaveLength(5);
  });

  it('renders question about data source', () => {
    render(<Faq />);

    expect(screen.getByText(/where does the data come from/i)).toBeInTheDocument();
  });

  it('renders question about coverage', () => {
    render(<Faq />);

    expect(screen.getByText(/which areas do you cover/i)).toBeInTheDocument();
  });

  it('renders question about free tier', () => {
    render(<Faq />);

    expect(screen.getByText(/is there a free tier/i)).toBeInTheDocument();
  });

  it('renders question about community use', () => {
    render(<Faq />);

    expect(screen.getByText(/can communities use town crier/i)).toBeInTheDocument();
  });

  it('renders question about notification speed', () => {
    render(<Faq />);

    expect(screen.getByText(/how quickly will i be notified/i)).toBeInTheDocument();
  });

  it('renders answer text for each question', () => {
    const { container } = render(<Faq />);

    const details = container.querySelectorAll('details');
    details.forEach((detail) => {
      const summary = detail.querySelector('summary');
      expect(summary).toBeInTheDocument();

      // Each detail should have content beyond just the summary
      const answerText = detail.textContent!.replace(summary!.textContent!, '').trim();
      expect(answerText.length).toBeGreaterThan(0);
    });
  });
});
