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

describe('apple-app-site-association', () => {
  interface AasaComponent {
    '/': string;
  }

  interface AasaDetail {
    appIDs: string[];
    components: AasaComponent[];
  }

  interface Aasa {
    applinks: {
      details: AasaDetail[];
    };
  }

  function loadAasa(): Aasa {
    const aasaPath = resolve(__dirname, '../../public/.well-known/aasa.json');
    const raw = readFileSync(aasaPath, 'utf-8');
    return JSON.parse(raw) as Aasa;
  }

  it('declares the iOS app bundle ID with team prefix', () => {
    const aasa = loadAasa();

    expect(aasa.applinks.details[0]?.appIDs).toContain(
      '4574VQ7N2X.uk.towncrierapp.mobile',
    );
  });

  it('claims /applications and /applications/* paths only', () => {
    const aasa = loadAasa();

    const components = aasa.applinks.details[0]?.components ?? [];
    const paths = components.map((c) => c['/']);

    expect(paths).toEqual(['/applications', '/applications/*']);
  });
});

describe('staticwebapp.config.json — AASA route', () => {
  interface RouteRule {
    route: string;
    headers?: Record<string, string>;
    rewrite?: string;
  }

  interface ConfigWithRoutes {
    routes: RouteRule[];
  }

  function loadConfigWithRoutes(): ConfigWithRoutes {
    const configPath = resolve(__dirname, '../../public/staticwebapp.config.json');
    const raw = readFileSync(configPath, 'utf-8');
    return JSON.parse(raw) as ConfigWithRoutes;
  }

  it('rewrites apple-app-site-association to a .json file so SWA serves application/json', () => {
    // Apple's swcd daemon rejects the AASA file unless served as application/json.
    // Azure SWA infers Content-Type from the file extension and ignores routes[].headers
    // for extensionless files, so we rewrite the canonical path to a .json file
    // whose extension causes SWA to set application/json automatically.
    const config = loadConfigWithRoutes();

    const aasaRoute = config.routes.find(
      (r) => r.route === '/.well-known/apple-app-site-association',
    );

    expect(aasaRoute).toBeDefined();
    expect(aasaRoute?.rewrite).toBe('/.well-known/aasa.json');
  });

  it('ships the AASA payload at the rewrite target /.well-known/aasa.json', () => {
    const aasaJsonPath = resolve(__dirname, '../../public/.well-known/aasa.json');
    const raw = readFileSync(aasaJsonPath, 'utf-8');
    const parsed = JSON.parse(raw) as { applinks: { details: unknown[] } };

    expect(parsed.applinks.details.length).toBeGreaterThan(0);
  });
});

describe('assetlinks.json', () => {
  interface AssetLinksTarget {
    namespace: string;
    package_name: string;
    sha256_cert_fingerprint: string[];
  }

  interface AssetLinksEntry {
    relation: string[];
    target: AssetLinksTarget;
  }

  function loadAssetLinks(): AssetLinksEntry[] {
    const assetLinksPath = resolve(
      __dirname,
      '../../public/.well-known/assetlinks.json',
    );
    const raw = readFileSync(assetLinksPath, 'utf-8');
    return JSON.parse(raw) as AssetLinksEntry[];
  }

  it('grants delegate_permission/common.handle_all_urls to every entry', () => {
    const assetLinks = loadAssetLinks();

    expect(assetLinks.length).toBeGreaterThan(0);
    for (const entry of assetLinks) {
      expect(entry.relation).toContain('delegate_permission/common.handle_all_urls');
    }
  });

  it('targets the android_app namespace for every entry', () => {
    const assetLinks = loadAssetLinks();

    for (const entry of assetLinks) {
      expect(entry.target.namespace).toBe('android_app');
    }
  });

  it('declares the debug keystore SHA-256 fingerprint for uk.towncrierapp.mobile.dev', () => {
    const assetLinks = loadAssetLinks();
    const devEntry = assetLinks.find(
      (entry) => entry.target.package_name === 'uk.towncrierapp.mobile.dev',
    );

    expect(devEntry?.target.sha256_cert_fingerprint).toContain(
      '75:2F:87:AF:52:B6:4D:33:71:ED:77:2A:2A:1C:D9:7A:A4:67:9E:1A:17:F0:9F:FD:92:12:D6:55:92:FD:0E:07',
    );
  });
});
