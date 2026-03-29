import { useState, useEffect, useCallback, type DependencyList } from 'react';

interface FetchDataState<T> {
  data: T | null;
  isLoading: boolean;
  error: string | null;
}

interface FetchDataResult<T> extends FetchDataState<T> {
  refresh: () => void;
}

interface FetchDataOptions {
  enabled?: boolean;
}

function extractErrorMessage(err: unknown): string {
  return err instanceof Error ? err.message : 'An error occurred';
}

export function useFetchData<T>(
  fetcher: () => Promise<T>,
  deps: DependencyList,
  options?: FetchDataOptions,
): FetchDataResult<T> {
  const enabled = options?.enabled ?? true;

  const [state, setState] = useState<FetchDataState<T>>({
    data: null,
    isLoading: enabled,
    error: null,
  });

  const load = useCallback(async () => {
    setState(prev => ({ ...prev, isLoading: true, error: null }));
    try {
      const data = await fetcher();
      setState({ data, isLoading: false, error: null });
    } catch (err: unknown) {
      setState(prev => ({ ...prev, isLoading: false, error: extractErrorMessage(err) }));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    (async () => {
      try {
        const data = await fetcher();
        if (!cancelled) {
          setState({ data, isLoading: false, error: null });
        }
      } catch (err: unknown) {
        if (!cancelled) {
          setState(prev => ({ ...prev, isLoading: false, error: extractErrorMessage(err) }));
        }
      }
    })();
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, ...deps]);

  return { ...state, refresh: load };
}
