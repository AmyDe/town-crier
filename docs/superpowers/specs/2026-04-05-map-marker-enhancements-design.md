# Map Marker Enhancements

Date: 2026-04-05

## Overview

Enhance the map view with color-coded markers distinguishing saved from unsaved applications, save/unsave capability from marker popups, and auto-fit bounds to show all visible pins.

## Data Flow

The map currently fetches all applications via user authorities. It needs to also know which applications are saved.

- `useMapData` gains a `savedApplicationUids: Set<string>` alongside the existing `applications` array
- Fetch saved apps via the existing `/v1/me/saved-applications` endpoint in parallel with authority-based fetching
- Expose `saveApplication(uid)` and `unsaveApplication(uid)` mutation functions
- Mutations are optimistic: update the local `Set` immediately, then fire the API call. On failure, revert and surface an error.

### Port Changes

`MapPort` needs two new methods:

- `fetchSavedApplications(): Promise<readonly SavedApplication[]>`
- `saveApplication(uid: ApplicationUid): Promise<void>`
- `unsaveApplication(uid: ApplicationUid): Promise<void>`

These delegate to the existing `savedApplicationsApi` module which already wraps these endpoints.

## Markers

Replace default Leaflet markers with custom SVG-based `L.DivIcon` markers:

- **Saved:** Amber (`--tc-amber`, `#F59E0B`) teardrop pin with a filled bookmark glyph
- **Unsaved:** Slate grey (`#94A3B8`) teardrop pin, no inner glyph or subtle outline
- SVG rendered inline via `L.DivIcon` with `html` property -- no extra image assets needed
- Icon size and anchor point should match Leaflet's default marker dimensions (25x41, anchor at 12x41)

## Popup

- Existing content stays: description, address, "View details" link
- Add a bookmark icon button in the top-right corner of the popup
- Filled bookmark = saved, outline bookmark = unsaved
- Tapping the icon toggles the save state
- Marker color updates immediately (optimistic update)
- The bookmark button should have an accessible aria-label ("Save application" / "Unsave application")

## Auto-fit Bounds

- After applications load, compute a `L.LatLngBounds` from all markable applications
- Use a `FitBounds` child component (following the existing `FitToCircle` pattern in `ConfirmMap.tsx`) that calls `map.fitBounds(bounds, { padding: [50, 50] })`
- Falls back to UK center (`[51.5074, -0.1278]`) at zoom 13 when there are no applications
- Bounds are recalculated only when the application list changes, not on every render

## Component Structure

```
ConnectedMapPage          # Creates adapter, passes port
  MapPage                 # Receives port, manages data via useMapData
    MapContainer          # Leaflet map
      TileLayer           # OSM tiles
      FitBounds           # Auto-fit child component
      MapMarker[]         # Custom marker + popup per application
        Popup
          BookmarkButton  # Save/unsave toggle
```

`MapMarker` is a thin wrapper that selects the correct icon based on saved state and renders the popup with the bookmark button.

## Testing

- `useMapData` tests: verify saved UIDs are fetched and merged, optimistic save/unsave updates the set, failure reverts
- `MapPage` tests: verify markers render with correct icon type based on saved state, bookmark button calls save/unsave, FitBounds is rendered when applications exist
- Spy implementations for the new port methods
