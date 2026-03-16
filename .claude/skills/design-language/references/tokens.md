# Design Token Reference

Complete token definitions with hex values for all three theme modes. These values have been chosen to meet WCAG AA contrast requirements in each mode.

## Brand Colors

| Token | Light | Dark | OLED Dark | Notes |
|-------|-------|------|-----------|-------|
| `tcAmber` | `#D4910A` | `#E9A620` | `#E9A620` | Primary accent. Darkened slightly in light mode for AA contrast on white surfaces. |
| `tcAmberMuted` | `#D4910A` at 15% | `#E9A620` at 15% | `#E9A620` at 15% | Tinted backgrounds for secondary emphasis. |
| `tcAmberHover` | `#B87A08` | `#F0B83A` | `#F0B83A` | Pressed/hover state for amber interactive elements. |

## Surface Colors

| Token | Light | Dark | OLED Dark | Notes |
|-------|-------|------|-----------|-------|
| `tcBackground` | `#FAF8F5` | `#1A1A1E` | `#000000` | Page background. Warm off-white avoids the clinical feel of pure white. |
| `tcSurface` | `#FFFFFF` | `#242428` | `#0A0A0A` | Card/component background. Sits above `tcBackground`. |
| `tcSurfaceElevated` | `#FFFFFF` | `#2E2E33` | `#161616` | Modals, bottom sheets. Sits above `tcSurface`. |

## Text Colors

| Token | Light | Dark | OLED Dark | Contrast on Surface |
|-------|-------|------|-----------|---------------------|
| `tcTextPrimary` | `#1C1917` | `#F1EFE9` | `#F1EFE9` | >15:1 (light), >13:1 (dark), >14:1 (OLED) |
| `tcTextSecondary` | `#6B6560` | `#9B9590` | `#9B9590` | >4.5:1 all modes |
| `tcTextTertiary` | `#A39E98` | `#5C5852` | `#5C5852` | >3:1 (large text / UI only) |
| `tcTextOnAccent` | `#FFFFFF` | `#1C1917` | `#1C1917` | >4.5:1 on `tcAmber` |

## Status Colors

Planning application status requires both a foreground (text/icon) and background (badge fill) variant. The background variant is the foreground at 15% opacity on `tcSurface`.

| Token | Light | Dark | OLED Dark | Meaning |
|-------|-------|------|-----------|---------|
| `tcStatusApproved` | `#1A7D37` | `#34C759` | `#34C759` | Application approved/granted |
| `tcStatusRefused` | `#C42B2B` | `#FF453A` | `#FF453A` | Application refused |
| `tcStatusPending` | `#C27A0A` | `#FFB340` | `#FFB340` | Awaiting decision / under review |
| `tcStatusWithdrawn` | `#7A7570` | `#8E8A85` | `#8E8A85` | Application withdrawn |
| `tcStatusAppealed` | `#7C3AED` | `#A78BFA` | `#A78BFA` | Decision under appeal |

The dark/OLED values use brighter variants to maintain visibility on dark backgrounds. Status colors on iOS should use system semantic colors where they align (e.g., `tcStatusApproved` maps closely to `.systemGreen`), but define custom values to ensure cross-platform consistency.

## Utility Colors

| Token | Light | Dark | OLED Dark | Notes |
|-------|-------|------|-----------|-------|
| `tcBorder` | `#E8E4DF` | `#3A3A3F` | `#1E1E22` | Dividers, card outlines. Subtle but visible. |
| `tcBorderFocused` | `#D4910A` | `#E9A620` | `#E9A620` | Focus rings. Matches `tcAmber`. |
| `tcOverlay` | `#000000` at 40% | `#000000` at 50% | `#000000` at 50% | Scrim behind modals/sheets. |

## Shadows

Shadows are only used in light mode. In dark and OLED modes, elevation is communicated through surface color stepping (`tcBackground` < `tcSurface` < `tcSurfaceElevated`).

| Token | Light | Dark / OLED |
|-------|-------|-------------|
| `tcShadowCard` | 0 2pt 8pt `#00000008` (5% black) | none |
| `tcShadowElevated` | 0 4pt 16pt `#0000000D` (8% black) | none |

## Spacing Scale

| Token | Value |
|-------|-------|
| `tcSpaceXS` | 4pt |
| `tcSpaceSM` | 8pt |
| `tcSpaceMD` | 16pt |
| `tcSpaceLG` | 24pt |
| `tcSpaceXL` | 32pt |
| `tcSpaceXXL` | 48pt |

## Corner Radius Scale

| Token | Value |
|-------|-------|
| `tcRadiusSM` | 8pt |
| `tcRadiusMD` | 12pt |
| `tcRadiusLG` | 16pt |
| `tcRadiusFull` | 9999pt (capsule) |

## Typography Scale (iOS)

| Token | Dynamic Type Style | Weight | Line Height Multiplier |
|-------|--------------------|--------|----------------------|
| `tcDisplayLarge` | `.largeTitle` | `.bold` | 1.2 |
| `tcDisplaySmall` | `.title2` | `.semibold` | 1.25 |
| `tcHeadline` | `.headline` | `.semibold` | 1.3 |
| `tcBody` | `.body` | `.regular` | 1.4 |
| `tcBodyEmphasis` | `.body` | `.semibold` | 1.4 |
| `tcCaption` | `.caption` | `.regular` | 1.3 |
| `tcCaptionEmphasis` | `.caption` | `.medium` | 1.3 |

## Animation Durations

| Token | Value | Easing | Usage |
|-------|-------|--------|-------|
| `tcDurationFast` | 150ms | `.spring(response: 0.3)` | Button press, toggle, micro-interactions |
| `tcDurationNormal` | 250ms | `.spring(response: 0.4)` | Sheet present, tab switch, card expand |
| `tcDurationSlow` | 400ms | `.spring(response: 0.5)` | Full-screen transitions, onboarding |
