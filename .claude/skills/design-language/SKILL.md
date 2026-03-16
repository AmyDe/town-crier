---
name: design-language
description: "Town Crier cross-platform design system — colors, typography, spacing, components, and theming (light, dark, OLED dark). MUST consult this skill before creating or modifying ANY UI code across iOS, Android, or web. Trigger whenever: building SwiftUI views, defining colors or themes, creating UI components, laying out screens, choosing typography or spacing, implementing dark mode, designing new features, reviewing UI code, or discussing visual design. Also trigger when the user mentions look and feel, branding, design tokens, theming, color palette, or visual consistency. Do NOT use for backend API code, infrastructure, CI/CD, or non-visual architecture decisions."
---

# Town Crier Design Language

## Design Philosophy

Town Crier presents complex planning data to a broad audience — residents, community groups, and property professionals. The design must make that data feel approachable without dumbing it down. Three principles guide every decision:

1. **Calm clarity.** A neutral canvas with generous white space lets content breathe. Each screen has one hero element (a map, a status badge, an application summary) that the eye lands on instantly. Supporting detail is available but subordinate — progressive disclosure, not information overload.

2. **Purposeful color.** Color is never decorative. It communicates: application status (approved, refused, pending), interactive affordances (tappable elements), and brand identity (the warm amber accent). Outside of those roles, the interface stays neutral.

3. **Trustworthy warmth.** The app deals with property and planning — topics that feel bureaucratic. The design counters that with rounded shapes, friendly typography weight, and warm neutral tones. It should feel like a helpful neighbour who happens to know planning law, not a government portal.

These principles are inspired by the design approaches of Monzo (warm brand color on a clean canvas, card-based progressive disclosure), Apple Podcasts (content-forward layouts, strong typographic hierarchy, restrained chrome), and Ivy (generous white space, single hero metric per screen, data made simple).

## Theme Modes

The app supports three appearance modes. All UI code must respect the active theme — never hard-code colors.

| Mode | Background | When |
|------|-----------|------|
| **Light** | Warm off-white canvas | System light appearance (default) |
| **Dark** | Charcoal surfaces | System dark appearance |
| **OLED Dark** | True black (#000000) | User toggle within dark mode settings |

OLED Dark is a sub-mode of Dark — it activates only when the system is in dark mode AND the user enables "True Black" in settings. The toggle exists because OLED screens save battery with pure black pixels, and some users strongly prefer it.

## Color System

Colors are defined as semantic tokens, not raw values. Every color has a light, dark, and OLED dark variant. Code should reference tokens (e.g., `Color.tcBackground`) and never use hex literals in views.

For the full token table with hex values and usage notes, read `references/tokens.md`.

### Semantic Palette Overview

**Brand**
- `tcAmber` — Primary accent. Used for CTAs, selected states, key interactive elements. A warm gold that evokes town crier bells and announcements.
- `tcAmberMuted` — Lower-emphasis accent. Used for secondary buttons, subtle highlights, tags.

**Surfaces**
- `tcBackground` — Page-level background. Warm off-white (light), charcoal (dark), pure black (OLED).
- `tcSurface` — Card and component background. White (light), elevated charcoal (dark), near-black (OLED).
- `tcSurfaceElevated` — Modals, bottom sheets, popovers. Slightly elevated above `tcSurface`.

**Text**
- `tcTextPrimary` — Body text, headings. Near-black with warm undertone (light), off-white (dark/OLED).
- `tcTextSecondary` — Captions, metadata, timestamps. Medium grey, legible but recessed.
- `tcTextTertiary` — Placeholder text, disabled labels. Light grey.
- `tcTextOnAccent` — Text rendered on `tcAmber` backgrounds. Must pass WCAG AA contrast.

**Status** (planning application lifecycle)
- `tcStatusApproved` — Green. Application granted.
- `tcStatusRefused` — Red. Application refused.
- `tcStatusPending` — Amber/orange. Under review or awaiting decision.
- `tcStatusWithdrawn` — Grey. No longer active.
- `tcStatusAppealed` — Purple. Decision under appeal.

**Utility**
- `tcBorder` — Subtle dividers and card outlines.
- `tcBorderFocused` — Focus rings and active input borders. Uses `tcAmber`.
- `tcOverlay` — Semi-transparent scrim behind modals and bottom sheets.

### Accessibility Requirements

- All text/background combinations must meet **WCAG AA** (4.5:1 for body text, 3:1 for large text and UI components).
- Status colors must not be the sole indicator — always pair with an icon or label. ~8% of men have some form of color vision deficiency.
- The token values in `references/tokens.md` have been chosen to meet these contrast ratios across all three theme modes.

## Typography

### iOS
Use **SF Pro** (the system font) via SwiftUI's `.system()` modifier. This ensures Dynamic Type support, proper optical sizing, and platform consistency.

### Android (future)
Use **Inter** — a geometric sans-serif designed for screens, with optical sizing and a large x-height that pairs naturally with SF Pro.

### Web (future)
Use **Inter** via Google Fonts or self-hosted.

### Type Scale

The scale uses a consistent set of semantic roles. Sizes are specified as iOS Dynamic Type styles — these automatically scale with the user's accessibility settings.

| Token | iOS Style | Weight | Usage |
|-------|----------|--------|-------|
| `tcDisplayLarge` | `.largeTitle` | Bold | Screen titles, hero numbers |
| `tcDisplaySmall` | `.title2` | Semibold | Section headers |
| `tcHeadline` | `.headline` | Semibold | Card titles, list row primary text |
| `tcBody` | `.body` | Regular | Body text, descriptions |
| `tcBodyEmphasis` | `.body` | Semibold | Inline emphasis, key values |
| `tcCaption` | `.caption` | Regular | Timestamps, metadata, secondary info |
| `tcCaptionEmphasis` | `.caption` | Medium | Status labels, badges |

Never use `.font(.system(size:))` with a numeric point size — this creates text that ignores the user's accessibility settings. Every piece of text, including icons used as decorative elements and placeholder labels, must use a Dynamic Type text style (`.largeTitle`, `.title`, `.headline`, `.body`, `.caption`, etc.). If you need a large decorative icon, use `.font(.system(.largeTitle))` or `.imageScale(.large)` rather than a fixed point size.

### Typographic Hierarchy

Each screen should use at most 3 levels of the type scale. Hierarchy is achieved through weight contrast (bold vs regular) and size contrast, not through color variation or decoration. Bold for the hero element, regular for supporting text, caption for metadata.

## Spacing & Layout

### Spacing Scale

A 4pt base unit provides rhythm without being too rigid. All spacing, padding, and margins should use multiples of this base.

| Token | Value | Usage |
|-------|-------|-------|
| `tcSpaceXS` | 4pt | Tight gaps — icon-to-label, inline elements |
| `tcSpaceSM` | 8pt | Compact padding — within dense components |
| `tcSpaceMD` | 16pt | Standard padding — card insets, list row padding |
| `tcSpaceLG` | 24pt | Section gaps — between card groups |
| `tcSpaceXL` | 32pt | Major sections — screen-level vertical rhythm |
| `tcSpaceXXL` | 48pt | Hero spacing — top of screen breathing room |

### Layout Principles

- **Content width:** Maximum 600pt content width on larger screens (iPad, future web). Centered with `tcSpaceMD` horizontal margins.
- **Card insets:** `tcSpaceMD` (16pt) on all sides.
- **List row height:** Minimum 44pt tap target (Apple HIG). Vertical padding of `tcSpaceSM` (8pt) above and below content.
- **Section spacing:** `tcSpaceLG` (24pt) between distinct content groups.
- **Safe areas:** Always respect system safe areas. Never let content sit under the notch, home indicator, or status bar.

## Corner Radius

| Token | Value | Usage |
|-------|-------|-------|
| `tcRadiusSM` | 8pt | Small elements — badges, chips, input fields |
| `tcRadiusMD` | 12pt | Cards, buttons, list groupings |
| `tcRadiusLG` | 16pt | Bottom sheets, modals, large cards |
| `tcRadiusFull` | Capsule | Pills, tags, rounded action buttons |

The rounded shape language signals approachability. Sharp corners are reserved for full-bleed edges (screen edges, navigation bars).

## Component Patterns

### Cards
Cards are the primary container for planning application summaries and grouped information.

- Background: `tcSurface`
- Corner radius: `tcRadiusMD`
- Padding: `tcSpaceMD` on all sides
- Shadow: subtle drop shadow in light mode (0pt x 2pt blur 8pt, 5% black). No shadow in dark/OLED modes — use `tcBorder` instead.
- Cards should not be nested inside other cards.

### Status Badges
A compact indicator showing planning application status.

- Shape: `tcRadiusFull` (capsule)
- Background: the relevant `tcStatus*` color at 15% opacity
- Text: the relevant `tcStatus*` color at full opacity, `tcCaptionEmphasis` weight
- Always include a status icon alongside the text label (for colour-blind accessibility)

### Buttons
Primary actions use `tcAmber` with `tcTextOnAccent` text. Secondary actions use `tcSurface` with `tcTextPrimary` text and a `tcBorder` outline. Both use `tcRadiusMD`.

- Minimum tap target: 44pt height
- Horizontal padding: `tcSpaceMD`
- Disabled state: 40% opacity

### Lists
Transaction-feed style lists (inspired by Monzo's activity feed) for planning application timelines.

- Row padding: `tcSpaceMD` horizontal, `tcSpaceSM` vertical
- Dividers: `tcBorder`, inset from the leading edge (aligned with text, not icons)
- Swipe actions: use platform-native swipe patterns

### Bottom Sheets
For filters, detail views, and secondary actions.

- Background: `tcSurfaceElevated`
- Corner radius: `tcRadiusLG` on top corners only
- Grab handle: centered, `tcTextTertiary` color
- Scrim behind: `tcOverlay`

### Empty States
When a list or screen has no content, show a centered illustration or icon with a brief explanation and a CTA.

- Icon: `tcTextTertiary` color, 48pt
- Title: `tcHeadline`
- Description: `tcBody`, `tcTextSecondary`
- CTA button: primary style

## Iconography

- **iOS:** SF Symbols exclusively. They scale with Dynamic Type, support all rendering modes, and look native.
- **Android (future):** Material Symbols — the closest equivalent to SF Symbols in weight and optical sizing.
- **Web (future):** Phosphor Icons or Material Symbols — both offer variable weight and optical sizing.

Icon usage:
- Navigation and tab bar icons: medium weight, 24pt
- Inline with text: match the text weight visually
- Status indicators: always paired with a text label
- Decorative/empty state: light weight, larger sizes (32-48pt)

## Motion & Animation

Keep animations functional, not decorative. Motion should help the user understand spatial relationships and state changes.

- **Duration:** 250ms for most transitions (sheet presentations, tab switches). 150ms for micro-interactions (button press, toggle).
- **Easing:** Use platform-default spring animations on iOS (SwiftUI's `.spring()` or `.default`). Avoid linear easing — it feels mechanical.
- **What to animate:** Screen transitions, sheet presentations, status changes, loading states.
- **What NOT to animate:** Text changes, list reordering (unless the user initiated it), background color transitions between themes.

## Platform Implementation Notes

### iOS (SwiftUI)

Define all design tokens as extensions on SwiftUI types in the `town-crier-presentation` package. Theme-aware colors should use `Color(uiColor:)` with a custom `UIColor` that resolves based on `UITraitCollection.userInterfaceStyle` and an app-level "OLED mode" flag from `UserDefaults` / `@AppStorage`.

For the OLED toggle:
- Store the preference using `@AppStorage("oledDarkEnabled")` — not raw `UserDefaults`. `@AppStorage` integrates with SwiftUI's observation system so views automatically re-render when the toggle changes, without manual `objectWillChange` plumbing or `didSet` observers.
- The color resolver checks: if system appearance is dark AND oledDarkEnabled is true, use OLED values; if system appearance is dark, use dark values; otherwise, use light values.
- The toggle should appear in Settings only when the system is in dark mode (it's meaningless in light mode).

Read `references/tokens.md` for the complete token definitions with hex values per theme.

### Android & Web (future)

These platforms will define the same semantic tokens using their native mechanisms (Jetpack Compose theme / CSS custom properties). The values and naming conventions are defined here to ensure consistency when those platforms are built. The token names (`tcBackground`, `tcAmber`, etc.) should be identical across all platforms.
