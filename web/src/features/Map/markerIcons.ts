import L from 'leaflet';

const SAVED_COLOR = '#E9A620';
const UNSAVED_COLOR = '#94A3B8';

function pinSvg(fill: string, inner: string): string {
  return `<svg viewBox="0 0 25 41" width="25" height="41" xmlns="http://www.w3.org/2000/svg">
    <path d="M12.5 0C5.6 0 0 5.6 0 12.5C0 21.9 12.5 41 12.5 41S25 21.9 25 12.5C25 5.6 19.4 0 12.5 0Z" fill="${fill}"/>
    ${inner}
  </svg>`;
}

export const savedMarkerIcon = L.divIcon({
  html: pinSvg(SAVED_COLOR, '<path d="M9 7h7v11l-3.5-2.5L9 18V7z" fill="white"/>'),
  className: '',
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
});

export const unsavedMarkerIcon = L.divIcon({
  html: pinSvg(UNSAVED_COLOR, '<circle cx="12.5" cy="12.5" r="4.5" fill="white" fill-opacity="0.9"/>'),
  className: '',
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
});
