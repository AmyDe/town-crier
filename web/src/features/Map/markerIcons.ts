import L from 'leaflet';

/**
 * Maps a PlanIt `app_state` to the design-system status colour token used to
 * tint a single-member pin. Mirrors the ApplicationCard status treatment so the
 * map and list agree. Unknown states fall back to the neutral withdrawn token.
 */
const STATUS_TOKEN: Record<string, string> = {
  Undecided: '--tc-status-pending',
  Permitted: '--tc-status-permitted',
  Conditions: '--tc-status-conditions',
  Rejected: '--tc-status-rejected',
  Withdrawn: '--tc-status-withdrawn',
  Appealed: '--tc-status-appealed',
  Unresolved: '--tc-status-withdrawn',
  Referred: '--tc-status-appealed',
  'Not Available': '--tc-status-withdrawn',
};

function statusToken(appState: string): string {
  return STATUS_TOKEN[appState] ?? '--tc-status-withdrawn';
}

/**
 * HTML for an amber aggregate count bubble (`count > 1` cell). Styling lives in
 * `leaflet-overrides.css` (`.tc-cluster-bubble`); the amber token is referenced
 * inline because Leaflet divIcons inject raw HTML outside the CSS-module scope.
 */
export function countBubbleHtml(count: number): string {
  return `<div class="tc-cluster-bubble" style="background: var(--tc-amber)">${count}</div>`;
}

/**
 * HTML for a status-coloured map pin (`count == 1` cell). The per-status colour
 * is passed as the `--tc-pin-color` custom property so `.tc-status-pin` in
 * `leaflet-overrides.css` can theme the SVG fill from a design token.
 */
export function statusPinHtml(appState: string): string {
  const token = statusToken(appState);
  return `<div class="tc-status-pin" style="--tc-pin-color: var(${token})">
    <svg viewBox="0 0 25 41" width="25" height="41" xmlns="http://www.w3.org/2000/svg">
      <path d="M12.5 0C5.6 0 0 5.6 0 12.5C0 21.9 12.5 41 12.5 41S25 21.9 25 12.5C25 5.6 19.4 0 12.5 0Z"/>
      <circle cx="12.5" cy="12.5" r="4.5" fill="white" fill-opacity="0.9"/>
    </svg>
  </div>`;
}

export function countBubbleIcon(count: number): L.DivIcon {
  return L.divIcon({
    html: countBubbleHtml(count),
    className: 'tc-cluster-bubble-wrapper',
    iconSize: [36, 36],
    iconAnchor: [18, 18],
  });
}

export function statusPinIcon(appState: string): L.DivIcon {
  return L.divIcon({
    html: statusPinHtml(appState),
    className: 'tc-status-pin-wrapper',
    iconSize: [25, 41],
    iconAnchor: [12, 41],
  });
}
