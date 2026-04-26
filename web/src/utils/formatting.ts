export function formatDate(isoDate: string): string {
  const date = new Date(isoDate);
  return date.toLocaleDateString('en-GB', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  });
}

const STATUS_CLASS_MAP: Record<string, string> = {
  Undecided: 'statusUndecided',
  Permitted: 'statusPermitted',
  Conditions: 'statusConditions',
  Rejected: 'statusRejected',
  Withdrawn: 'statusWithdrawn',
  Appealed: 'statusAppealed',
  Unresolved: 'statusUnresolved',
  Referred: 'statusReferred',
  'Not Available': 'statusNotAvailable',
};

export function statusClassName(
  appState: string,
  styles: Record<string, string | undefined>,
): string {
  const key = STATUS_CLASS_MAP[appState];
  if (key !== undefined) {
    return styles[key] ?? '';
  }
  return styles['statusDefault'] ?? '';
}

/**
 * Maps a PlanIt `app_state` wire string to the user-facing label residents
 * expect to see. UK residents talk about applications being "Granted" or
 * "Refused" rather than "Permitted" or "Rejected", so this layer translates
 * the wire vocabulary to friendly copy. Unknown values pass through unchanged.
 */
const STATUS_DISPLAY_LABEL_MAP: Record<string, string> = {
  Permitted: 'Granted',
  Conditions: 'Granted with conditions',
  Rejected: 'Refused',
};

export function statusDisplayLabel(appState: string): string {
  return STATUS_DISPLAY_LABEL_MAP[appState] ?? appState;
}
