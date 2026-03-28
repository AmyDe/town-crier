import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

interface NavigationFallback {
  rewrite: string;
  exclude: string[];
}

interface GlobalHeaders {
  [key: string]: string;
}

interface StaticWebAppConfig {
  navigationFallback: NavigationFallback;
  globalHeaders: GlobalHeaders;
}

function loadConfig(): StaticWebAppConfig {
  const configPath = resolve(__dirname, '../../public/staticwebapp.config.json');
  const raw = readFileSync(configPath, 'utf-8');
  return JSON.parse(raw) as StaticWebAppConfig;
}

describe('staticwebapp.config.json', () => {
  describe('navigationFallback', () => {
    it('rewrites to /index.html for SPA routing', () => {
      const config = loadConfig();

      expect(config.navigationFallback.rewrite).toBe('/index.html');
    });

    it('excludes static asset paths from fallback', () => {
      const config = loadConfig();

      expect(config.navigationFallback.exclude).toContain(
        '*.{css,js,svg,png,jpg,jpeg,gif,ico,woff,woff2,ttf,eot,json,txt}',
      );
    });
  });

  describe('globalHeaders', () => {
    it('sets X-Content-Type-Options to nosniff', () => {
      const config = loadConfig();

      expect(config.globalHeaders['X-Content-Type-Options']).toBe('nosniff');
    });

    it('sets X-Frame-Options to DENY', () => {
      const config = loadConfig();

      expect(config.globalHeaders['X-Frame-Options']).toBe('DENY');
    });

    it('sets Referrer-Policy to strict-origin-when-cross-origin', () => {
      const config = loadConfig();

      expect(config.globalHeaders['Referrer-Policy']).toBe(
        'strict-origin-when-cross-origin',
      );
    });
  });
});

describe('robots.txt', () => {
  it('allows all user agents', () => {
    const robotsPath = resolve(__dirname, '../../public/robots.txt');
    const content = readFileSync(robotsPath, 'utf-8');

    expect(content).toContain('User-agent: *');
    expect(content).toContain('Allow: /');
  });
});
