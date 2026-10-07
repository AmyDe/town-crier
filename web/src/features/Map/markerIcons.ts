import L from 'leaflet';

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

/** The amber token is inline because Leaflet divIcons inject raw HTML outside CSS-module scope. */
export function countBubbleHtml(count: number): string {
  return `<div class="tc-cluster-bubble" style="background: var(--tc-amber)">${count}</div>`;
}

/** The status colour is passed as `--tc-pin-color` so `.tc-status-pin` can theme the SVG fill. */
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
