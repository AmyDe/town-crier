# Watch Zone Radius Slider

## Context

The watch zone editor uses a SwiftUI segmented `Picker` for radius selection (`WatchZoneEditorView.swift:91-100`). With 5–8 tier-dependent options compressed into the available width, every label truncates ("50…", "1.5…", "10…") and the control looks broken. A horizontal slider is both more functional (continuous-feeling adjustment) and more legible (one large live label instead of eight tiny ones).

## Design

Replace the segmented `Picker` with a SwiftUI `Slider` over a 100 m step from 100 m to the tier's `maxRadiusMetres`. Above the slider, render the current value at `tcBodyEmphasis` weight using the existing `formatRadius()` formatter ("2 km", "1.5 km", "750 m"). Below the slider, render the min and max as `tcCaption` / `tcTextSecondary` end labels so users can see the range at a glance.

The slider tint uses `tcAmber` (matches the existing selected-state colour). Step granularity is uniform 100 m throughout — simple to implement, gives 19–99 stops depending on tier, and the live label keeps every position legible. SwiftUI's built-in haptic snap on stepped sliders handles the "muscle memory" affordance.

Tier-based max is unchanged: Free 2 km, Personal 5 km, Pro 10 km. The slider's `in:` range is `100...limits.maxRadiusMetres`. `WatchZoneLimits.availableRadiusOptions` (the discrete list) is no longer used by the editor; leave it in place for now since `RadiusPickerStepView` (onboarding) still references it — see Out of scope.

### Accessibility

- `.accessibilityValue(formatRadius(selectedRadiusMetres))` so VoiceOver announces "2 kilometres" rather than the raw double.
- `.accessibilityLabel("Radius")`.
- SwiftUI's `Slider` provides increment/decrement adjustable actions automatically when given a `step:`, so no custom `accessibilityAdjustableAction` is needed — verify in the test that VoiceOver step matches the visual step.
- Dynamic Type: the live label uses `TCTypography.bodyEmphasis` (a Dynamic Type style), so it scales with the user's text size setting.
- Tap target: the slider thumb is platform-default (44 pt hit area).

## Scope

**In:**
- Replace `radiusSection` in `WatchZoneEditorView.swift` with a slider + live label + min/max end labels.
- Tests covering: initial value renders, dragging updates `selectedRadiusMetres` in 100 m steps, slider range respects tier max, accessibility value matches `formatRadius()`.

**Out:**
- `RadiusPickerStepView` (onboarding) — different layout (vertical list, no truncation), file separately if we want consistency.
- Changing the tier max values themselves.
- Domain-level minimum radius (currently `> 0`); keep as is — the UI floor of 100 m is a presentation concern.
- Map preview behaviour — already reactive to `selectedRadiusMetres`, no changes needed.

## Steps

### Slider component
Replace the `Picker` block with a `VStack` containing: the live formatted value (`tcBodyEmphasis`, `tcTextPrimary`), a `Slider(value: $viewModel.selectedRadiusMetres, in: 100...viewModel.maxRadiusMetres, step: 100)` tinted `tcAmber`, and an `HStack` with the formatted min and max labels.

### ViewModel surface
Expose `maxRadiusMetres: Double` on `WatchZoneEditorViewModel` (delegating to `limits.maxRadiusMetres`) so the View doesn't reach into `limits` directly. `availableRadiusOptions` stays for now (used by onboarding).

### Snap-on-edit safety
When editing an existing zone whose `radiusMetres` is not a 100 m multiple (e.g. data from before this change), the initial slider position should display correctly and snap to the nearest 100 m only when the user actually moves it. SwiftUI's `Slider` with `step:` already behaves this way — confirm in a test.

### Tests
- `WatchZoneEditorViewTests` (SwiftUI snapshot or `Inspector` if used elsewhere — match existing convention): slider renders with correct range and initial value for each tier.
- `WatchZoneEditorViewModelTests`: `maxRadiusMetres` reflects the tier passed in.
- `RadiusFormattingTests`: existing tests already cover the formatter; no change needed.
