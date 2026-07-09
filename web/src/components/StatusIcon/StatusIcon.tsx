/**
 * Small decorative glyph paired with every status "stamp" across the app
 * (ApplicationCard, ApplicationDetailPage, SearchResultCard) so status is
 * never conveyed by colour alone (design-language accessibility requirement —
 * ~8% of men have some form of colour vision deficiency). Purely decorative:
 * `aria-hidden`, no text content — the accessible label lives in the caller
 * alongside it.
 *
 * A minimal hand-rolled glyph set, consistent with this codebase's existing
 * inline-SVG icons (Navbar hamburger, ThemeToggle sun/moon) — not the full
 * icon system, which is out of scope here (epic #848 R7).
 */

export type StatusIconGlyph = 'pending' | 'granted' | 'rejected' | 'withdrawn' | 'appealed';

const GLYPH_BY_APP_STATE: Record<string, StatusIconGlyph> = {
  Undecided: 'pending',
  Unresolved: 'pending',
  'Not Available': 'pending',
  Permitted: 'granted',
  Conditions: 'granted',
  Rejected: 'rejected',
  Withdrawn: 'withdrawn',
  Appealed: 'appealed',
  Referred: 'appealed',
};

function glyphForAppState(appState: string): StatusIconGlyph {
  return GLYPH_BY_APP_STATE[appState] ?? 'withdrawn';
}

interface Props {
  appState: string;
  className?: string;
}

export function StatusIcon({ appState, className }: Props) {
  const glyph = glyphForAppState(appState);

  return (
    <svg
      data-testid="status-icon"
      data-icon={glyph}
      className={className}
      width="12"
      height="12"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {glyph === 'pending' && (
        <>
          <circle cx="8" cy="8" r="6.25" />
          <path d="M8 4.5V8l2.5 1.5" />
        </>
      )}
      {glyph === 'granted' && <path d="M3.5 8.5 6.5 11.5 12.5 4.5" />}
      {glyph === 'rejected' && (
        <>
          <path d="M4 4 12 12" />
          <path d="M12 4 4 12" />
        </>
      )}
      {glyph === 'withdrawn' && <path d="M4 8H12" />}
      {glyph === 'appealed' && (
        <>
          <path d="M4 12 12 4" />
          <path d="M6 4H12V10" />
        </>
      )}
    </svg>
  );
}
