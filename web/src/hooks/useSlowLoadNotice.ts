import { useEffect, useState } from 'react';

/** True once a load of a large zone has run for `delayMs`; resets when the load, the zone or the delay changes. */
export function useSlowLoadNotice(
  isLoading: boolean,
  isLarge: boolean,
  delayMs = 3000,
): boolean {
  const [hasElapsed, setHasElapsed] = useState(false);

  useEffect(() => {
    if (!isLoading || !isLarge) return;
    const timer = setTimeout(() => setHasElapsed(true), delayMs);
    return () => {
      clearTimeout(timer);
      setHasElapsed(false);
    };
  }, [isLoading, isLarge, delayMs]);

  return isLoading && isLarge && hasElapsed;
}
