// GENERATED FILE — edit design/tokens.json and run scripts/design-tokens/generate.mjs
// Consumed by web/scripts/lib/render-shared.mjs pageStyles(). See ADR 0040.

export const SEO_TOKEN_CSS = `    /* GENERATED — design tokens from design/tokens.json; run scripts/design-tokens/generate.mjs */
    :root {
      /* Background scale converged with the share-page family (see
         api-go/internal/sharepage/templates/styles.gohtml): a warm cream field
         (--tc-background: #FAF8F5) behind a white card (--tc-surface:
         #FFFFFF) in light mode, so SEO pages and share pages read as one
         product rather than two differently-themed properties. */
      --tc-amber: #D4910A;
      --tc-amber-hover: #B87A08;
      --tc-background: #FAF8F5;
      --tc-surface: #FFFFFF;
      --tc-text-primary: #1C1917;
      --tc-text-secondary: #6B6560;
      --tc-text-on-accent: #FFFFFF;
      /* Shared status chip vocabulary (decision 4, punch-list #794): three
         canonical buckets — granted (green), refused (red), neutral (the
         undecided/long-tail catch-all) — converged with the design-language
         tcStatusApproved/tcStatusRefused/tcTextSecondary tokens, replacing the
         previous five-way ad-hoc per-appState palette. The *-bg tokens are the
         foreground colour at 15% opacity, for the filled badge style. */
      --tc-status-granted: #1A7D37;
      --tc-status-granted-bg: #1A7D3726;
      --tc-status-refused: #C42B2B;
      --tc-status-refused-bg: #C42B2B26;
      --tc-status-neutral: #6B6560;
      --tc-status-neutral-bg: #6B656026;
      --tc-border: #E8E4DF;
      --tc-radius-md: 12px;
      --tc-radius-full: 9999px;
      --tc-space-sm: 8px;
      --tc-space-md: 16px;
      --tc-space-lg: 24px;
      --tc-space-xl: 32px;
      --tc-space-xxl: 48px;
      --tc-font-family: 'Inter', system-ui, -apple-system, sans-serif;
      --tc-content-max-width: 760px;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --tc-amber: #E9A620;
        --tc-amber-hover: #F0B83A;
        --tc-background: #1A1A1E;
        --tc-surface: #242428;
        --tc-text-primary: #F1EFE9;
        --tc-text-secondary: #9B9590;
        --tc-text-on-accent: #1C1917;
        --tc-status-granted: #34C759;
        --tc-status-granted-bg: #34C75926;
        --tc-status-refused: #FF453A;
        --tc-status-refused-bg: #FF453A26;
        --tc-status-neutral: #9B9590;
        --tc-status-neutral-bg: #9B959026;
        --tc-border: #3A3A3F;
      }
    }`;
