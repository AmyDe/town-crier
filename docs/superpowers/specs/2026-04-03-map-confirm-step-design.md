# Map Preview on Watch Zone Confirmation

Date: 2026-04-03

## Problem

When creating a watch zone (during onboarding or from the watch zones tab), the confirm step shows raw latitude/longitude decimals (e.g. `51.5007, -0.1246`). This is meaningless to users — they can't verify whether the geocode landed in the right place or visualise their coverage area.

The watch zone creation flow has no confirmation step at all — it saves immediately after the user enters a name and radius.

## Solution

Replace coordinate text with an interactive Leaflet map showing a marker at the geocoded location and a translucent circle representing the selected radius. Display the user's entered postcode as a human-readable label. Apply this to both the onboarding and watch zone creation flows.

## Components

### `ConfirmMap` (new shared component)

**Location:** `web/src/components/ConfirmMap/ConfirmMap.tsx`

**Props:**
- `latitude: number`
- `longitude: number`
- `radiusMetres: number`

**Behaviour:**
- Renders a `MapContainer` with `TileLayer` (OSM tiles, same URL as `MapPage`)
- Places a `Marker` at the geocoded coordinates
- Draws a `Circle` overlay with the given radius
- Auto-fits zoom to contain the full circle with padding (using Leaflet `fitBounds` on the circle's bounding box)
- Pan/zoom enabled, but marker is not draggable and circle is not resizable
- Fixed height (~250px), fills card width

**Circle styling:**
- Fill: `rgba(74, 108, 247, 0.15)` (semi-transparent primary blue)
- Stroke: `rgba(74, 108, 247, 0.8)`, weight 2

### `PostcodeInput` callback change

The `onGeocode` callback signature changes to include the postcode string:

```typescript
// Before
onGeocode: (result: GeocodeResult) => void

// After
onGeocode: (result: GeocodeResult, postcode: string) => void
```

The `PostcodeInput` component already has the postcode value in local state — it just needs to pass it through.

### Onboarding flow changes

**`useOnboarding` hook:**
- Add `postcode: string` state, set when `handleGeocode` fires
- Expose `postcode` in the return value

**`OnboardingPage` confirm step:**
- Replace the coordinate row with `<ConfirmMap>` component
- Show postcode below the map (e.g. "SW1A 1AA")
- Keep radius text row as-is
- Confirm button unchanged

**Updated flow:** `welcome → postcode → radius → confirm (with map + postcode label) → done`

### Watch zone creation flow changes

**`useCreateWatchZone` hook:**
- Add `postcode: string` state, set when `setGeocode` fires
- Change step type from `'postcode' | 'details'` to `'postcode' | 'details' | 'confirm'`
- Add `confirmDetails` callback that transitions from `details` to `confirm`
- Expose `postcode` in the return value

**`WatchZoneCreatePage` confirm step:**
- Details step: "Save" button becomes "Next", calls `confirmDetails`
- New confirm step shows:
  - `<ConfirmMap>` component
  - Postcode label
  - Zone name
  - Radius text
  - "Confirm" button that calls `save()`

**Updated flow:** `postcode → details (name + radius) → confirm (with map) → save`

## What stays the same

- All existing onboarding steps (welcome, postcode, radius)
- The `finish()` / `save()` API calls
- No new npm dependencies — `react-leaflet` and `leaflet` are already installed
- No backend changes
- `MapPage` is unaffected

## Testing

- `ConfirmMap` — renders without crashing with valid props
- `OnboardingPage` — verify postcode is displayed (not coordinates) on confirm step
- `WatchZoneCreatePage` — verify new confirm step appears with map before save
- `useCreateWatchZone` — verify `details → confirm` step transition
