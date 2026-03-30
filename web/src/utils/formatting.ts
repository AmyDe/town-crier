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
  Approved: 'statusApproved',
  Refused: 'statusRefused',
  Withdrawn: 'statusWithdrawn',
  Appealed: 'statusAppealed',
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
