# Design Token Reference

Complete token definitions for all three theme modes (light, dark, OLED dark). Colour values are chosen to meet WCAG AA contrast in each mode.

**Colours are generated.** The value tables below (between the `tokens:generated` markers) are emitted from the single source of truth, `design/tokens.json`, by `scripts/design-tokens/generate.mjs` (ADR 0040). Do not hand-edit them — edit `tokens.json` and regenerate. A CI drift gate (`scripts/check-design-token-drift.sh`) fails any PR that changes the tokens without regenerating. The same source drives the web sheet, the iOS `Color+TownCrier.swift`, and the Android `Color.kt`, which is why the three platforms can no longer drift apart.

**Names differ per platform.** Web exposes CSS custom properties (`--tc-*`); iOS a `Color` extension (`Color.tc*`) plus `TCSpacing`/`TCCornerRadius` enums; Android a `TcPalette` (`TcPalette.*`) plus `TownCrierSpacing`/`TownCrierRadius` objects. Each table lists every platform's real name and a **Platforms** column, because not every token exists everywhere — `amber-hover` is web + Android only (iOS has no hover state), and the `full` radius, shadows and motion durations are web-only or web + Android (see each table).

## Colours

Status colours follow the PlanIt vocabulary (Permitted / Conditions / Rejected). Each status colour is used both as a foreground (text/icon) and, at 15% opacity on `surface`, as a badge fill. The dark/OLED values are brighter variants so they stay legible on dark backgrounds.

<!-- tokens:generated:begin -->
### Brand

| Token | Web | iOS | Android | Light | Dark | OLED | Platforms |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `amber` | `--tc-amber` | `Color.tcAmber` | `TcPalette.amber` | `#9E6709` | `#E9A620` | `#E9A620` | Web, iOS, Android |
| `amber-muted` | `--tc-amber-muted` | `Color.tcAmberMuted` | `TcPalette.amberMuted` | `#9E6709 @ 12%` | `#E9A620 @ 15%` | `#E9A620 @ 15%` | Web, iOS, Android |
| `amber-hover` | `--tc-amber-hover` | — | `TcPalette.amberHover` | `#8A5F06` | `#F0B83A` | `#F0B83A` | Web, Android |

### Surface

| Token | Web | iOS | Android | Light | Dark | OLED | Platforms |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `background` | `--tc-background` | `Color.tcBackground` | `TcPalette.background` | `#F5F0E6` | `#191713` | `#000000` | Web, iOS, Android |
| `surface` | `--tc-surface` | `Color.tcSurface` | `TcPalette.surface` | `#FFFDF6` | `#23201A` | `#0A0908` | Web, iOS, Android |
| `surface-elevated` | `--tc-surface-elevated` | `Color.tcSurfaceElevated` | `TcPalette.surfaceElevated` | `#FFFDF6` | `#2B2822` | `#151310` | Web, iOS, Android |

### Text

| Token | Web | iOS | Android | Light | Dark | OLED | Platforms |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `text-primary` | `--tc-text-primary` | `Color.tcTextPrimary` | `TcPalette.textPrimary` | `#24201A` | `#EFE9DC` | `#EFE9DC` | Web, iOS, Android |
| `text-secondary` | `--tc-text-secondary` | `Color.tcTextSecondary` | `TcPalette.textSecondary` | `#6D665C` | `#A69E8F` | `#A69E8F` | Web, iOS, Android |
| `text-tertiary` | `--tc-text-tertiary` | `Color.tcTextTertiary` | `TcPalette.textTertiary` | `#A79E90` | `#6E675B` | `#6E675B` | Web, iOS, Android |
| `text-on-accent` | `--tc-text-on-accent` | `Color.tcTextOnAccent` | `TcPalette.textOnAccent` | `#FFFDF8` | `#1C1917` | `#1C1917` | Web, iOS, Android |

### Status

| Token | Web | iOS | Android | Light | Dark | OLED | Platforms |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `status-permitted` | `--tc-status-permitted` | `Color.tcStatusPermitted` | `TcPalette.statusPermitted` | `#1A7D37` | `#34C759` | `#34C759` | Web, iOS, Android |
| `status-conditions` | `--tc-status-conditions` | `Color.tcStatusConditions` | `TcPalette.statusConditions` | `#B85C00` | `#FF9F0A` | `#FF9F0A` | Web, iOS, Android |
| `status-rejected` | `--tc-status-rejected` | `Color.tcStatusRejected` | `TcPalette.statusRejected` | `#C42B2B` | `#FF453A` | `#FF453A` | Web, iOS, Android |
| `status-pending` | `--tc-status-pending` | `Color.tcStatusPending` | `TcPalette.statusPending` | `#C27A0A` | `#FFB340` | `#FFB340` | Web, iOS, Android |
| `status-withdrawn` | `--tc-status-withdrawn` | `Color.tcStatusWithdrawn` | `TcPalette.statusWithdrawn` | `#7A7570` | `#8E8A85` | `#8E8A85` | Web, iOS, Android |
| `status-appealed` | `--tc-status-appealed` | `Color.tcStatusAppealed` | `TcPalette.statusAppealed` | `#7C3AED` | `#A78BFA` | `#A78BFA` | Web, iOS, Android |

### Utility

| Token | Web | iOS | Android | Light | Dark | OLED | Platforms |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `border` | `--tc-border` | `Color.tcBorder` | `TcPalette.border` | `#DAD2C2` | `#3A352B` | `#26221A` | Web, iOS, Android |
| `border-focused` | `--tc-border-focused` | `Color.tcBorderFocused` | `TcPalette.borderFocused` | `#9E6709` | `#E9A620` | `#E9A620` | Web, iOS, Android |
| `overlay` | `--tc-overlay` | `Color.tcOverlay` | `TcPalette.overlay` | `#000000 @ 40%` | `#000000 @ 50%` | `#000000 @ 50%` | Web, iOS, Android |

### Spacing

| Token | Web | iOS | Android | Value | Platforms |
| --- | --- | --- | --- | --- | --- |
| `xs` | `--tc-space-xs` | `TCSpacing.extraSmall` | `TownCrierSpacing.xs` | 4pt | Web, iOS, Android |
| `sm` | `--tc-space-sm` | `TCSpacing.small` | `TownCrierSpacing.sm` | 8pt | Web, iOS, Android |
| `md` | `--tc-space-md` | `TCSpacing.medium` | `TownCrierSpacing.md` | 16pt | Web, iOS, Android |
| `lg` | `--tc-space-lg` | `TCSpacing.large` | `TownCrierSpacing.lg` | 24pt | Web, iOS, Android |
| `xl` | `--tc-space-xl` | `TCSpacing.extraLarge` | `TownCrierSpacing.xl` | 32pt | Web, iOS, Android |
| `xxl` | `--tc-space-xxl` | `TCSpacing.extraExtraLarge` | `TownCrierSpacing.xxl` | 48pt | Web, iOS, Android |

### Corner radius

| Token | Web | iOS | Android | Value | Platforms |
| --- | --- | --- | --- | --- | --- |
| `sm` | `--tc-radius-sm` | `TCCornerRadius.small` | `TownCrierRadius.sm` | 3pt | Web, iOS, Android |
| `md` | `--tc-radius-md` | `TCCornerRadius.medium` | `TownCrierRadius.md` | 6pt | Web, iOS, Android |
| `lg` | `--tc-radius-lg` | `TCCornerRadius.large` | `TownCrierRadius.lg` | 12pt | Web, iOS, Android |
| `full` | `--tc-radius-full` | — | `TownCrierRadius.full` | capsule | Web, Android |

### Shadows

Web only. Dark and OLED communicate elevation through surface stepping, not shadow.

| Token | Web | Light | Dark / OLED | Platforms |
| --- | --- | --- | --- | --- |
| `card` | `--tc-shadow-card` | `0 1px 3px rgba(0, 0, 0, 0.06)` | `none` | Web |
| `elevated` | `--tc-shadow-elevated` | `0 4px 16px rgba(0, 0, 0, 0.08)` | `none` | Web |

### Motion durations

Web only.

| Token | Web | Value | Platforms |
| --- | --- | --- | --- |
| `fast` | `--tc-duration-fast` | 150ms | Web |
| `normal` | `--tc-duration-normal` | 250ms | Web |
| `slow` | `--tc-duration-slow` | 400ms | Web |
<!-- tokens:generated:end -->

## Typography

All four surfaces use one sans-serif family for every role, including headings. A Fraunces display-serif treatment shipped briefly with the Public Notice rebrand (GH#857) and was removed everywhere on 2026-07-10 after TestFlight tester feedback favoured the consistency of a single typeface over the serif's character (GH#912).

Typography is hand-maintained per platform — it is **not** generated from `tokens.json` (the scales are stable single files; only the colour matrix drifts in practice, ADR 0040). Web uses the `--tc-text-*` / `--tc-weight-*` / `--tc-leading-*` custom properties; the two mobile platforms map named roles onto the system type stack so text respects the user's Dynamic Type / font-size setting.

### iOS (`TCTypography`)

All roles build on a Dynamic Type text style — `displayLarge`/`displaySmall`/`headline` add a `.weight()` override rather than pointing at a custom font, so Dynamic Type scaling is never broken.

| Token | Dynamic Type role | Weight |
|-------|-------------------|--------|
| `TCTypography.displayLarge` | `.largeTitle` | `.semibold` |
| `TCTypography.displaySmall` | `.title2` | `.semibold` |
| `TCTypography.headline` | `.headline` | `.semibold` |
| `TCTypography.body` | `.body` | `.regular` |
| `TCTypography.bodyEmphasis` | `.body` | `.semibold` |
| `TCTypography.caption` | `.caption` | `.regular` |
| `TCTypography.captionEmphasis` | `.caption` | `.medium` |
| `TCTypography.mono` | `.caption`, monospaced design | `.regular` |
| `TCTypography.monoEmphasis` | `.caption`, monospaced design | `.medium` |

### Android (`TownCrierTypography`)

Town Crier's type scale maps each tc token onto a Material 3 role; every role — including the three display/headline roles — uses `InterFontFamily`. Sizes and line-heights stay at the M3 role defaults.

| tc token | Material 3 role | Weight |
|----------|-----------------|--------|
| `tcDisplayLarge` | `headlineLarge` | `SemiBold` |
| `tcDisplaySmall` | `titleLarge` | `SemiBold` |
| `tcHeadline` | `titleMedium` | `SemiBold` |
| `tcBody` | `bodyLarge` | `Regular` |
| `tcBodyEmphasis` | `bodyLarge` | `SemiBold` |
| `tcCaption` | `bodySmall` | `Regular` |
| `tcCaptionEmphasis` | `labelMedium` | `Medium` |
| `tcMono` (`TownCrierTheme.mono`) | — (built off `bodySmall`) | `Regular` |

`TownCrierTheme.mono` has no dedicated M3 role — it copies `bodySmall`'s metrics with the system monospace family and tabular figures (`fontFeatureSettings = "tnum"`).

### Web (CSS custom properties)

| Custom property | Value | Used by |
|------------------|-------|---------|
| `--tc-font-family` | `'Inter', system-ui, -apple-system, sans-serif` | Body copy (default) |
| `--tc-font-display` | `'Inter', system-ui, -apple-system, sans-serif` (same stack as `--tc-font-family`) | `Hero` title, `ApplicationCard`'s filed-notice title |
| `--tc-font-mono` | `ui-monospace, 'SF Mono', Menlo, 'Roboto Mono', monospace` | `.tc-mono-meta` utility class (`global.css`) — references, dates, distances |

Before 2026-07-10, `--tc-font-display` held a self-hosted Fraunces stack; it now duplicates `--tc-font-family` since the display role is sans everywhere. The `--tc-text-*` / `--tc-weight-*` size and weight scale is unaffected by the font-family change.

### Email (digest, Go)

`api-go/internal/digest/email.go` inlines font stacks directly — email clients can't load the self-hosted webfonts the other surfaces use.

| Constant | Value |
|----------|-------|
| `bodyFontStack` | `-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif` |
| `headlineFontStack` | alias of `bodyFontStack` (previously its own `Georgia, 'Times New Roman', serif` stand-in for Fraunces) |
| `monoFontStack` | `'Courier New', monospace` — unchanged |
