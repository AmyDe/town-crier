import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { PostcodeInput } from '../PostcodeInput';
import { SpyGeocodingPort } from './spies/spy-geocoding-port';

describe('PostcodeInput', () => {
  it('renders an input and a look up button', () => {
    const spy = new SpyGeocodingPort();

    render(<PostcodeInput geocodingPort={spy} onGeocode={() => {}} />);

    expect(screen.getByRole('textbox', { name: /postcode/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /look up/i })).toBeInTheDocument();
  });

  it('calls onGeocode with coordinates after a successful lookup', async () => {
    const spy = new SpyGeocodingPort();
    spy.geocodeResult = { latitude: 51.5074, longitude: -0.1278 };
    const handleGeocode = vi.fn();
    const user = userEvent.setup();

    render(<PostcodeInput geocodingPort={spy} onGeocode={handleGeocode} />);

    await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'SW1A 1AA');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    await waitFor(() => {
      expect(handleGeocode).toHaveBeenCalledWith({ latitude: 51.5074, longitude: -0.1278 }, 'SW1A 1AA');
    });
  });

  it('shows an error message on geocode failure', async () => {
    const spy = new SpyGeocodingPort();
    spy.geocodeError = new Error('Postcode not found');
    const user = userEvent.setup();

    render(<PostcodeInput geocodingPort={spy} onGeocode={() => {}} />);

    await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'INVALID');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Postcode not found');
    });
  });

  it('disables the button while geocoding is in progress', async () => {
    const spy = new SpyGeocodingPort();
    let resolveGeocode: (value: { latitude: number; longitude: number }) => void;
    spy.geocode = (postcode: string) => {
      spy.geocodeCalls.push(postcode);
      return new Promise((resolve) => {
        resolveGeocode = resolve;
      });
    };
    const user = userEvent.setup();

    render(<PostcodeInput geocodingPort={spy} onGeocode={() => {}} />);

    await user.type(screen.getByRole('textbox', { name: /postcode/i }), 'CB1 2AD');
    await user.click(screen.getByRole('button', { name: /look up/i }));

    expect(screen.getByRole('button', { name: /look up/i })).toBeDisabled();

    resolveGeocode!({ latitude: 52.2, longitude: 0.12 });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /look up/i })).toBeEnabled();
    });
  });
});
