const DEFAULT_FALLBACK = 'An error occurred';

export function extractErrorMessage(
  err: unknown,
  fallback: string = DEFAULT_FALLBACK,
): string {
  return err instanceof Error ? err.message : fallback;
}
