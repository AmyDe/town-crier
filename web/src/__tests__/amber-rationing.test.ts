import { describe, it, expect } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

/**
 * Public Notice component spec (epic #848) rations `var(--tc-amber)` to five
 * roles: primary CTA, active nav/tab state, focus ring, unread-rule, and the
 * paid-tier upsell card. Everywhere else that referenced amber before this
 * sweep must now use a text/border token instead. Each case below extracts
 * the specific (non-allowed) rule block and asserts it no longer contains
 * amber, without touching the sibling rules that legitimately keep it (e.g.
 * `.chipPressed`, `.active`, focus rings, CTA buttons).
 */

const srcDir = resolve(__dirname, '..');

function readCss(relativePath: string): string {
  return readFileSync(resolve(srcDir, relativePath), 'utf-8');
}

function block(css: string, selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return css.match(new RegExp(`${escaped}\\s*\\{[^}]*\\}`, 's'))?.[0] ?? '';
}

describe('Amber rationing sweep — non-CTA/active/focus/unread/upsell uses converted', () => {
  it('SavedApplicationsPage: chip hover no longer uses amber (chipPressed active state keeps it)', () => {
    const css = readCss('features/SavedApplications/SavedApplicationsPage.module.css');
    expect(block(css, '.chip:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.chipPressed')).toContain('var(--tc-amber)');
  });

  it('SettingsPage: secondary button hover, legal link, and select hover no longer use amber', () => {
    const css = readCss('features/Settings/SettingsPage.module.css');
    expect(block(css, '.secondaryButton:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.legalLink')).not.toContain('var(--tc-amber)');
    expect(block(css, '.select:hover')).not.toContain('var(--tc-amber)');
  });

  it('RedeemOfferCode: success panel and reset-button hover no longer use amber (redeem CTA keeps it)', () => {
    const css = readCss('features/offerCode/RedeemOfferCode.module.css');
    expect(block(css, '.success')).not.toContain('var(--tc-amber)');
    expect(block(css, '.resetButton:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.button')).toContain('var(--tc-amber)');
  });

  it('SearchPage: notice banner no longer uses amber', () => {
    const css = readCss('features/Search/SearchPage.module.css');
    expect(block(css, '.notice')).not.toContain('var(--tc-amber)');
  });

  it('MapPage: chip hover no longer uses amber (chipActive filter state keeps it)', () => {
    const css = readCss('features/Map/MapPage.module.css');
    expect(block(css, '.chip:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.chipActive')).toContain('var(--tc-amber)');
  });

  it('ApplicationsPage: chip hover, mark-all-read, and load-more hover no longer use amber (chipPressed keeps it)', () => {
    const css = readCss('features/Applications/ApplicationsPage.module.css');
    expect(block(css, '.chip:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.markAllReadButton')).not.toContain('var(--tc-amber)');
    expect(block(css, '.markAllReadButton:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.loadMoreButton:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.chipPressed')).toContain('var(--tc-amber)');
  });

  it('Sidebar: wordmark and nav-link hover no longer use amber (active nav state keeps it)', () => {
    const css = readCss('components/Sidebar/Sidebar.module.css');
    expect(block(css, '.appName')).not.toContain('var(--tc-amber)');
    expect(block(css, '.appName:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.navLink:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.active')).toContain('var(--tc-amber)');
  });

  it('AppShell: hamburger hover and mobile title no longer use amber', () => {
    const css = readCss('components/AppShell/AppShell.module.css');
    expect(block(css, '.hamburger:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.mobileTitle')).not.toContain('var(--tc-amber)');
  });

  it('ThemeToggle: hover no longer uses amber', () => {
    const css = readCss('components/ThemeToggle/ThemeToggle.module.css');
    expect(block(css, '.button:hover')).not.toContain('var(--tc-amber)');
  });

  it('Pagination: button hover no longer uses amber', () => {
    const css = readCss('components/Pagination/Pagination.module.css');
    expect(block(css, '.button:hover:not(:disabled)')).not.toContain('var(--tc-amber)');
  });

  it('RadiusPicker: label hover no longer uses amber (checked/selected state keeps it)', () => {
    const css = readCss('components/RadiusPicker/RadiusPicker.module.css');
    expect(block(css, '.label:hover')).not.toContain('var(--tc-amber)');
    expect(block(css, '.radio:checked + .label')).toContain('var(--tc-amber)');
  });

  it('LargeRadiusWarning: warning callout no longer uses brand amber (uses the conditions/warning token)', () => {
    const css = readCss('components/LargeRadiusWarning/LargeRadiusWarning.module.css');
    expect(css).not.toContain('var(--tc-amber');
    expect(css).toContain('var(--tc-status-conditions)');
  });

  it('FullPageLoader: spinner accent no longer uses amber', () => {
    const css = readCss('components/FullPageLoader/FullPageLoader.module.css');
    expect(block(css, '.spinner')).not.toContain('var(--tc-amber)');
  });

  it('DashboardPage: quick-link hover no longer uses amber', () => {
    const css = readCss('features/Dashboard/DashboardPage.module.css');
    expect(block(css, '.quickLink:hover')).not.toContain('var(--tc-amber)');
  });

  it('WatchZoneEditPage: back link no longer uses amber (save CTA and focus ring keep it)', () => {
    const css = readCss('features/WatchZones/WatchZoneEditPage.module.css');
    expect(block(css, '.backLink')).not.toContain('var(--tc-amber)');
    expect(block(css, '.saveButton')).toContain('var(--tc-amber)');
  });
});
