import { describe, it, expect } from 'vitest';
import { readdirSync, statSync } from 'fs';
import { join, basename } from 'path';

/**
 * Convention test: all DI wrapper files (those that inject dependencies
 * into presentation components via useMemo + context hooks) must use
 * the "Connected" prefix, not "Wired".
 *
 * This enforces a single naming convention across the codebase.
 */
describe('DI wrapper naming convention', () => {
  const featuresDir = join(__dirname, '..', 'features');

  function findWiredFiles(dir: string): string[] {
    const results: string[] = [];
    const entries = readdirSync(dir);

    for (const entry of entries) {
      const fullPath = join(dir, entry);
      const stat = statSync(fullPath);

      if (stat.isDirectory()) {
        results.push(...findWiredFiles(fullPath));
      } else if (basename(entry).startsWith('Wired') && entry.endsWith('.tsx')) {
        results.push(fullPath);
      }
    }

    return results;
  }

  it('has no files using the Wired prefix — all DI wrappers use Connected', () => {
    const wiredFiles = findWiredFiles(featuresDir);

    expect(wiredFiles).toEqual([]);
  });

  it('has Connected wrapper files for all feature page wrappers', () => {
    function findConnectedFiles(dir: string): string[] {
      const results: string[] = [];
      const entries = readdirSync(dir);

      for (const entry of entries) {
        const fullPath = join(dir, entry);
        const stat = statSync(fullPath);

        if (stat.isDirectory()) {
          if (basename(fullPath) !== '__tests__') {
            results.push(...findConnectedFiles(fullPath));
          }
        } else if (basename(entry).startsWith('Connected') && entry.endsWith('.tsx')) {
          results.push(basename(entry));
        }
      }

      return results;
    }

    const connectedFiles = findConnectedFiles(featuresDir).sort();

    // All 13 DI wrapper files should use Connected prefix (Groups removed; SavedApplications re-introduced)
    expect(connectedFiles).toEqual([
      'ConnectedApplicationDetailPage.tsx',
      'ConnectedApplicationsPage.tsx',
      'ConnectedDashboardPage.tsx',
      'ConnectedLegalPage.tsx',
      'ConnectedMapPage.tsx',
      'ConnectedNotificationsPage.tsx',
      'ConnectedOnboardingPage.tsx',
      'ConnectedSavedApplicationsPage.tsx',
      'ConnectedSearchPage.tsx',
      'ConnectedSettingsPage.tsx',
      'ConnectedWatchZoneCreatePage.tsx',
      'ConnectedWatchZoneEditPage.tsx',
      'ConnectedWatchZoneListPage.tsx',
    ]);
  });
});
