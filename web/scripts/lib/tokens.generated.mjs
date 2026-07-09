// GENERATED FILE — edit design/tokens.json and run scripts/design-tokens/generate.mjs
// Consumed by web/scripts/lib/render-shared.mjs pageStyles(). See ADR 0040.

export const SEO_TOKEN_CSS = `    /* GENERATED — design tokens from design/tokens.json; run scripts/design-tokens/generate.mjs */
    :root {
      /* Background scale converged with the share-page family (see
         api-go/internal/sharepage/templates/styles.gohtml): a warm paper field
         (--tc-background) behind a warmer off-white card (--tc-surface) in light
         mode, so SEO pages and share pages read as one product rather than two
         differently-themed properties. */
      --tc-amber: #9E6709;
      --tc-amber-hover: #8A5F06;
      --tc-background: #F5F0E6;
      --tc-surface: #FFFDF6;
      --tc-text-primary: #24201A;
      --tc-text-secondary: #6D665C;
      --tc-text-on-accent: #FFFDF8;
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
      --tc-status-neutral: #6D665C;
      --tc-status-neutral-bg: #6D665C26;
      --tc-border: #DAD2C2;
      --tc-radius-md: 6px;
      --tc-radius-full: 9999px;
      --tc-space-sm: 8px;
      --tc-space-md: 16px;
      --tc-space-lg: 24px;
      --tc-space-xl: 32px;
      --tc-space-xxl: 48px;
      --tc-font-family: 'Inter', system-ui, -apple-system, sans-serif;
      --tc-font-display: 'Fraunces', 'Iowan Old Style', Georgia, serif;
      --tc-font-mono: ui-monospace, 'SF Mono', Menlo, 'Roboto Mono', monospace;
      --tc-content-max-width: 760px;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --tc-amber: #E9A620;
        --tc-amber-hover: #F0B83A;
        --tc-background: #191713;
        --tc-surface: #23201A;
        --tc-text-primary: #EFE9DC;
        --tc-text-secondary: #A69E8F;
        --tc-text-on-accent: #1C1917;
        --tc-status-granted: #34C759;
        --tc-status-granted-bg: #34C75926;
        --tc-status-refused: #FF453A;
        --tc-status-refused-bg: #FF453A26;
        --tc-status-neutral: #A69E8F;
        --tc-status-neutral-bg: #A69E8F26;
        --tc-border: #3A352B;
      }
    }`;
