# Hide Postcode Field on Watch Zone Edit Screen

Bead: tc-gdl0

## Problem

On the iOS watch zones screen, creating a zone takes a postcode, geocodes it to lat/long, and persists only the lat/long. When the user later taps a zone to edit it, the editor view renders the postcode `TextField` + "Look up postcode" button — but the field is empty, because the postcode itself was never stored. The UI implies an editable postcode that cannot actually be edited.

## Decision

Lock the zone's centre once created. The edit screen exposes only the name and radius. Users who want a different location delete the zone and create a new one.

This keeps the data model unchanged (no schema migration, no extra geocoding round-trips, no lossy reverse-geocoding) and matches the project owner's stated user model: re-locating an existing zone is not an envisioned flow.

Trade-off accepted: a user who renamed their zone to something like "Mum's house" will see no postcode anywhere on the edit screen. They can verify the location only via the name they chose. If signal later emerges that users want to see (or change) the original postcode, the cheapest follow-up is to add a `postcode` string to `WatchZoneDocument` and surface it on edit — that change does not depend on this one and is not blocked by it.

## Scope

### In scope

- iOS `WatchZoneEditorView`: hide the postcode `TextField` and the "Look up postcode" button when the editor is in edit mode.
- iOS `WatchZoneEditorViewModel`: no behavioural change. The init already pre-populates `geocodedCoordinate` from the existing zone's centre, so `save()` works without a geocoder round-trip.
- Tests covering both modes.

### Out of scope

- iOS `WatchZone` value object — no postcode field added.
- API `WatchZone` domain model — unchanged.
- API `WatchZoneDocument` (Cosmos) — unchanged.
- API `WatchZone.WithUpdates` — already supports name/radius updates; no API work.
- The auto-fill behaviour where `nameInput` defaults to the postcode value on create — kept.
- Removing `submitPostcode()` or `postcodeInput` from the view model. They remain part of the type's create-mode contract; making them conditional would make the view model harder to reason about. They simply become unreachable in edit mode.

## Behaviour

### Create mode (`isEditing == false`)

Unchanged. User types postcode → taps "Look up postcode" → coordinate appears → user adjusts name/radius → saves.

### Edit mode (`isEditing == true`)

The postcode `TextField` and "Look up postcode" button are not rendered. The user sees:
- The name field, pre-populated with the existing zone's name.
- The radius slider, pre-populated with the existing zone's radius.
- A save button.

`save()` builds a new `WatchZone` using the existing `geocodedCoordinate` (set in init from the zone's centre) and calls `repository.update(zone)` as today.

## Tests

### `WatchZoneEditorViewTests`

- New: in edit mode, the postcode field and lookup button are absent from the view hierarchy.
- Existing create-mode rendering tests stay green untouched.

### `WatchZoneEditorViewModelTests`

- New: in edit mode, `save()` succeeds without `submitPostcode()` having been called, and produces a `WatchZone` whose centre matches the original zone's centre.
- Existing create-mode tests stay green untouched.

## Non-goals

- No API changes.
- No Cosmos schema changes.
- No new geocoding behaviour.
- No reverse-geocoding lat/long → postcode.
- No persistence of postcode alongside lat/long.
