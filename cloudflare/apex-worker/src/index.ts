import { handle, type Env } from "./route";

export default {
  fetch(request, env, ctx) {
    return handle(request, env, {
      fetch: (input, init) => fetch(input, init),
      cache: caches.default,
      now: () => Date.now(),
      waitUntil: (promise) => ctx.waitUntil(promise),
    });
  },
} satisfies ExportedHandler<Env>;
