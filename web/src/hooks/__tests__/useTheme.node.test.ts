// @vitest-environment node
import { createElement } from 'react';
import { renderToString } from 'react-dom/server';
import { describe, it, expect } from 'vitest';
import { useTheme } from '../useTheme';

function ThemeProbe() {
  const { theme } = useTheme();
  return createElement('p', null, theme);
}

describe('useTheme without a DOM', () => {
  it('renders the light theme on the server', () => {
    expect(typeof window).toBe('undefined');

    expect(renderToString(createElement(ThemeProbe))).toBe('<p>light</p>');
  });
});
