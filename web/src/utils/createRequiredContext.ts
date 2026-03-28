import { createContext, useContext } from 'react';
import type { Context, Provider } from 'react';

export function createRequiredContext<T>(
  name: string,
): [Provider<T | null>, () => T] {
  const ctx: Context<T | null> = createContext<T | null>(null);

  function useRequiredContext(): T {
    const value = useContext(ctx);
    if (value === null) {
      throw new Error(`use${name} must be used within a ${name}.Provider`);
    }
    return value;
  }

  return [ctx.Provider, useRequiredContext];
}
